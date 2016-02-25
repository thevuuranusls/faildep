package slb

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	defaultMaxRetry = 0
	defaultMaxPick  = 3
)

var (
	AllServerDownError = fmt.Errorf("All Server Has Down")
	MaxRetryError      = fmt.Errorf("Max retry but still failure")
)

type repType int

const (
	ok repType = 1 << iota
	fail
	breakable
	retriable
)

type SLB struct {
	servers      nodeList
	distributor  distributor
	metrics      nodeMetrics
	maxRetry     uint
	maxRePick    uint
	repClassify  func(err error) repType
	retryBackOff func(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration
}

func NewSLB(addrs []string, trippedTimeoutFactor, failureThreshold uint, activeThreshold uint64, trippedTimeoutWindow, activeReqCountWindow time.Duration) *SLB {
	servers := make(nodeList, 0, len(addrs))
	for idx, addr := range addrs {
		servers = append(servers, Node{
			index:  idx,
			server: addr,
		})
	}
	m := newNodeMetric(
		servers,
		trippedTimeoutFactor,
		failureThreshold,
		activeThreshold,
		trippedTimeoutWindow,
		activeReqCountWindow,
	)
	d := newDistributor(addrs, m)
	return &SLB{
		servers:      servers,
		distributor:  *d,
		metrics:      *m,
		maxRetry:     defaultMaxRetry,
		maxRePick:    defaultMaxPick,
		repClassify:  networkErrorClassification,
		retryBackOff: decorrelatedJittered,
	}
}

func (l *SLB) Submit(service func(node *Node) error) error {

	execContext := &executionContext{}

	for execContext.serverAttemptCount <= l.maxRePick {

		execContext.incServerAttemptCount()

		execContext.node = l.distributor.pickServer(execContext.node, l.availableServer())
		if execContext.node == nil {
			return AllServerDownError
		}

		//fmt.Println("[Stat]|pick srv:", execContext.node.server)

		metric := l.metrics.takeMetric(*execContext.node)
		metric.incActive()
		for execContext.attemptCount <= l.maxRetry {
			execContext.incAttemptCount()
			startTime := time.Now()
			err := service(execContext.node)
			repType := l.repClassify(err)
			rt := time.Now().Sub(startTime)
			switch {
			case repType&ok == ok:
				metric.recordSuccess(rt)
				metric.descActive()
				return nil
			case repType&breakable == breakable:
				metric.recordFailure(rt)
			}
			backOffTime := l.retryBackOff(100*time.Millisecond, uint(2), 2*time.Second, execContext.attemptCount)
			fmt.Println("sleep:", backOffTime)
			time.Sleep(backOffTime)
		}
		execContext.resetAttemptCount()
		metric.descActive()
		backOffTime := l.retryBackOff(100*time.Millisecond, uint(2), 2*time.Second, execContext.attemptCount)
		fmt.Println("sleep:", backOffTime)
		time.Sleep(backOffTime)
	}

	return MaxRetryError
}

func (l *SLB) availableServer() nodeList {
	nodes := make([]Node, 0, len(l.servers))
	for _, node := range l.servers {
		if !l.metrics.isCircuitBreakTripped(node) && l.metrics.takeMetric(node).activeReqCount < l.metrics.activeThreshold {
			nodes = append(nodes, node)
		}
		//m := l.metrics.takeMetric(node)
		//fmt.Println("[Stat]|srv:", node.server, "|fail:", m.successiveFailCount, "|Concurrent:", m.takeActiveReqCount(), "|lastFailure:", m.lastFailedTimestamp)
	}
	return nodes
}

func networkErrorClassification(_err error) repType {
	var typ repType
	if _err == nil {
		typ |= ok
		return typ
	}
	switch t := _err.(type) {
	case net.Error:
		if t.Timeout() {
		}
		typ |= breakable
		return typ
	case *url.Error:
		if nestErr, ok := t.Err.(net.Error); ok {
			if nestErr.Timeout() {
			}
			typ |= breakable
			return typ
		}
	}
	if _err != nil && strings.Contains(_err.Error(), "use of closed network connection") {
		typ |= breakable
		return typ
	}
	return typ
}
