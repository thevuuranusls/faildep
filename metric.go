package faildep

import (
	"time"
)

type opType int

const (
	incSuccess opType = iota + 1
	incFailure
	incActive
	descActive
	getActive
	getMetric
	getFailCount
)

type op struct {
	typ opType
}

type resourceMetric struct {
	metrics                      *resourceMetrics
	successiveFailCount          uint
	activeReqCount               uint64
	lastFailedTimestamp          time.Time
	lastActiveReqCountChangeTime time.Time
	in                           chan op
	out                          chan interface{}
}

func (n *resourceMetric) loop(current time.Time) {
	for {
		select {
		case op := <-n.in:
			switch op.typ {
			case getMetric:
				n.out <- n
			case incFailure:
				n.successiveFailCount++
				n.lastFailedTimestamp = current
			case incSuccess:
				n.successiveFailCount = 0
			case incActive:
				n.activeReqCount++
				n.lastActiveReqCountChangeTime = current
			case descActive:
				if n.activeReqCount > 0 {
					n.activeReqCount--
				}
				n.lastActiveReqCountChangeTime = current
			case getActive:
				if n.activeReqCount == 0 {
					n.out <- n.activeReqCount
				} else if current.Sub(n.lastActiveReqCountChangeTime) > n.metrics.activeReqCountWindow || n.activeReqCount == 0 {
					n.activeReqCount = 0
					n.out <- 0
				} else {
					n.out <- n.activeReqCount
				}
			case getFailCount:
				n.out <- n.successiveFailCount

			}
		}
	}
}

type resourceMetrics struct {
	resources            func() ResourceList
	resChangeChan        chan struct{}
	metrics              map[Resource]*resourceMetric
	failureThreshold     uint
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

func (n *resourceMetrics) start() {
	current := time.Now()
	for _, metric := range n.metrics {
		go metric.loop(current)
	}
}

func (n *resourceMetrics) allServers() ResourceList {
	return n.resources()
}

func (n *resourceMetrics) availableServer(funcFlags funcFlag) ResourceList {
	servers := n.resources()
	nodes := make([]Resource, 0, len(servers))
	for _, node := range servers {
		if !(funcFlags&circuitBreaker == circuitBreaker && n.isCircuitBreakTripped(node)) &&
			!(funcFlags&bulkhead == bulkhead && n.takeMetric(node).activeReqCount >= n.activeThreshold) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (n *resourceMetric) recordSuccess(rt time.Duration) {
	n.in <- op{
		typ: incSuccess,
	}
}

func (n *resourceMetric) recordFailure(rt time.Duration) {
	n.in <- op{
		typ: incFailure,
	}
}

func (n *resourceMetrics) takeMetric(nd Resource) *resourceMetric {
	m := n.metrics[nd]
	m.in <- op{
		typ: getMetric,
	}
	rep := <-m.out
	return rep.(*resourceMetric)
}

func (n *resourceMetric) incActive() {
	n.in <- op{
		typ: incActive,
	}
}

func (n *resourceMetric) descActive() {
	n.in <- op{
		typ: descActive,
	}
}

func (n *resourceMetric) takeActiveReqCount() uint64 {
	n.in <- op{
		typ: getActive,
	}
	data := <-n.out
	return data.(uint64)
}

func (n *resourceMetric) takeFailCount() uint {
	n.in <- op{
		typ: getFailCount,
	}
	data := <-n.out
	return data.(uint)
}

func (n *resourceMetrics) takeCircuitBreakerTimeout(nd Resource) *time.Time {
	metric := n.takeMetric(nd)
	blackOutPeriod := n.takeCircuitBreakerBlackoutPeriod(metric)
	if blackOutPeriod <= 0 {
		return nil
	}
	timeout := metric.lastFailedTimestamp.Add(blackOutPeriod)
	return &timeout
}

func (n *resourceMetrics) isCircuitBreakTripped(nd Resource) bool {
	circuitBreakTimeout := n.takeCircuitBreakerTimeout(nd)
	if circuitBreakTimeout == nil {
		return false
	}
	return time.Now().Before(*circuitBreakTimeout)
}

func (n *resourceMetrics) takeCircuitBreakerBlackoutPeriod(m *resourceMetric) time.Duration {
	if m.successiveFailCount < n.failureThreshold {
		return 0 * time.Second
	}
	attempt := uint(m.successiveFailCount - n.failureThreshold)
	if attempt > 16 {
		attempt = 16
	}
	return n.trippedBackOff(n.trippedBaseTime, n.trippedTimeoutMax, attempt)
}
