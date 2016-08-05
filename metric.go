package faildep

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type opType int

type op struct {
	typ opType
}

type resourceMetrics struct {
	metricsLock          sync.RWMutex
	resources            func() ResourceList
	resChangeChan        chan struct{}
	metrics              map[Resource]*resourceMetric
	failureThreshold     uint64
	activeThreshold      uint64
	trippedBaseTime      time.Duration
	trippedTimeoutMax    time.Duration
	activeReqCountWindow time.Duration
	trippedBackOff       BackOff
}

func newNodeMetric(resources ResourceProvider) *resourceMetrics {
	res, c := resources()
	initSize := len(res())
	nm := &resourceMetrics{
		resources:      res,
		resChangeChan:  c,
		metrics:        make(map[Resource]*resourceMetric, initSize),
		trippedBackOff: Exponential,
	}
	return nm
}

func (n *resourceMetrics) allServers() ResourceList {
	return n.resources()
}

func (n *resourceMetrics) availableServer(funcFlags funcFlag) ResourceList {
	servers := n.resources()
	nodes := make([]Resource, 0, len(servers))
	for _, node := range servers {
		m := n.takeMetric(node)
		activeReqCount := atomic.LoadUint64(&m.activeReqCount)
		if !(funcFlags&circuitBreaker == circuitBreaker && m.isCircuitBreakTripped()) &&
			!(funcFlags&bulkhead == bulkhead && activeReqCount >= n.activeThreshold) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (n *resourceMetrics) takeMetric(nd Resource) *resourceMetric {
	n.metricsLock.Lock()
	m, ok := n.metrics[nd]
	if !ok {
		m = &resourceMetric{
			metrics:             n,
			successiveFailCount: 0,
			activeReqCount:      0,
		}
		n.metrics[nd] = m
	}
	n.metricsLock.Unlock()
	return m
}

func (n *resourceMetrics) takeCircuitBreakerBlackoutPeriod(successiveFailCount uint64) time.Duration {
	if successiveFailCount < n.failureThreshold {
		return 0 * time.Second
	}
	attempt := uint(successiveFailCount - n.failureThreshold)
	if attempt > 16 {
		attempt = 16
	}
	return n.trippedBackOff(n.trippedBaseTime, n.trippedTimeoutMax, attempt)
}

type resourceMetric struct {
	metrics                      *resourceMetrics
	successiveFailCount          uint64
	activeReqCount               uint64
	lastFailedTimestamp          unsafe.Pointer
	lastActiveReqCountChangeTime unsafe.Pointer
}

func (n *resourceMetric) recordSuccess(rt time.Duration) {
	atomic.StoreUint64(&n.successiveFailCount, 0)
}

func (n *resourceMetric) recordFailure(rt time.Duration) {
	current := time.Now()
	atomic.AddUint64(&n.successiveFailCount, 1)
	atomic.StorePointer(&n.lastFailedTimestamp, unsafe.Pointer(&current))
	return
}

func (n *resourceMetric) incActive() {
	current := time.Now()
	atomic.AddUint64(&n.activeReqCount, 1)
	atomic.StorePointer(&n.lastActiveReqCountChangeTime, unsafe.Pointer(&current))
}

func (n *resourceMetric) descActive() {
	current := time.Now()
	atomic.AddUint64(&n.activeReqCount, ^uint64(0))
	atomic.StorePointer(&n.lastActiveReqCountChangeTime, unsafe.Pointer(&current))
}

func (n *resourceMetric) takeActiveReqCount() uint64 {
	activeReqCount := atomic.LoadUint64(&n.activeReqCount)
	if activeReqCount == 0 {
		return 0
	}
	pt := atomic.LoadPointer(&n.lastActiveReqCountChangeTime)
	lastActiveReqCountChangeTime := (*time.Time)(pt)
	if time.Now().Sub(*lastActiveReqCountChangeTime) > n.metrics.activeReqCountWindow {
		atomic.StoreUint64(&n.activeReqCount, 0)
		return 0
	}
	return activeReqCount
}

func (n *resourceMetric) takeFailCount() uint64 {
	return atomic.LoadUint64(&n.successiveFailCount)
}

func (n *resourceMetric) isCircuitBreakTripped() bool {
	circuitBreakTimeout := n.takeCircuitBreakerTimeout()
	if circuitBreakTimeout == nil {
		return false
	}
	return time.Now().Before(*circuitBreakTimeout)
}

func (n *resourceMetric) takeCircuitBreakerTimeout() *time.Time {
	blackOutPeriod := n.metrics.takeCircuitBreakerBlackoutPeriod(n.takeFailCount())
	if blackOutPeriod <= 0 {
		return nil
	}
	pt := atomic.LoadPointer(&n.lastFailedTimestamp)
	lastFailedTimestamp := (*time.Time)(pt)
	timeout := lastFailedTimestamp.Add(blackOutPeriod)
	return &timeout
}
