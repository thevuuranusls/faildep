package slb

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
)

type op struct {
	typ opType
}

type nodeMetric struct {
	metrics                      *nodeMetrics
	successiveFailCount          uint
	activeReqCount               uint64
	lastFailedTimestamp          time.Time
	lastActiveReqCountChangeTime time.Time
	in                           chan op
	out                          chan interface{}
}

func (n *nodeMetric) loop(current time.Time) {
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
				n.activeReqCount--
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

			}
		}
	}
}

type nodeMetrics struct {
	metrics              map[Node]*nodeMetric
	failureThreshold     uint
	activeThreshold      uint64
	trippedTimeoutFactor uint
	trippedTimeoutWindow time.Duration
	activeReqCountWindow time.Duration
	trippedBackOff       func(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration
}

func newNodeMetric(nodes nodeList, trippedTimeoutFactor, failureThreshold uint, activeThreshold uint64,
	trippedTimeoutWindow, activeReqCountWindow time.Duration) *nodeMetrics {
	nm := &nodeMetrics{
		metrics:              make(map[Node]*nodeMetric, len(nodes)),
		trippedTimeoutFactor: trippedTimeoutFactor,
		failureThreshold:     failureThreshold,
		activeThreshold:      activeThreshold,
		trippedTimeoutWindow: trippedTimeoutWindow,
		activeReqCountWindow: activeReqCountWindow,
		trippedBackOff:       exponential,
	}
	for _, node := range nodes {
		metric := &nodeMetric{
			metrics:             nm,
			successiveFailCount: 0,
			activeReqCount:      0,
			in:                  make(chan op),
			out:                 make(chan interface{}),
		}
		go metric.loop(time.Now())
		nm.metrics[node] = metric

	}
	return nm
}

func (n *nodeMetric) recordSuccess(rt time.Duration) {
	n.in <- op{
		typ: incSuccess,
	}
}

func (n *nodeMetric) recordFailure(rt time.Duration) {
	n.in <- op{
		typ: incFailure,
	}
}

func (n *nodeMetrics) takeMetric(nd Node) *nodeMetric {
	m := n.metrics[nd]
	m.in <- op{
		typ: getMetric,
	}
	rep := <-m.out
	return rep.(*nodeMetric)
}

func (n *nodeMetric) incActive() {
	n.in <- op{
		typ: incActive,
	}
}

func (n *nodeMetric) descActive() {
	n.in <- op{
		typ: descActive,
	}
}

func (n *nodeMetric) takeActiveReqCount() uint64 {
	n.in <- op{
		typ: getActive,
	}
	data := <-n.out
	return data.(uint64)
}

func (n *nodeMetrics) takeCircuitBreakerTimeout(nd Node) *time.Time {
	metric := n.takeMetric(nd)
	blackOutPeriod := n.takeCircuitBreakerBlackoutPeriod(metric)
	if blackOutPeriod <= 0 {
		return nil
	}
	timeout := metric.lastFailedTimestamp.Add(blackOutPeriod)
	return &timeout
}

func (n *nodeMetrics) isCircuitBreakTripped(nd Node) bool {
	circuitBreakTimeout := n.takeCircuitBreakerTimeout(nd)
	if circuitBreakTimeout == nil {
		return false
	}
	return time.Now().Before(*circuitBreakTimeout)
}

func (n *nodeMetrics) takeCircuitBreakerBlackoutPeriod(m *nodeMetric) time.Duration {
	if m.successiveFailCount < n.failureThreshold {
		return 0 * time.Second
	}
	attempt := uint(m.successiveFailCount - n.failureThreshold)
	if attempt > 16 {
		attempt = 16
	}
	return n.trippedBackOff(1*time.Second, n.trippedTimeoutFactor, n.trippedTimeoutWindow, attempt)
}
