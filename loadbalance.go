package slb

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMaxRetry             = 0
	DefaultMaxPick              = 3
	DefaultTrippedTimeoutFactor = 2
	DefaultFailureThreshold     = 20
	DefaultActiveThreshold      = 50
	DefaultTrippedTimeoutMax    = 200 * time.Second
	DefaultActiveReqCountWindow = 1 * time.Second
)

var (
	// AllServerDownError returns when all backend server has down
	AllServerDownError = fmt.Errorf("All Server Has Down")
	// MaxRetryError returns when retry beyond given maxRetry time
	MaxRetryError = fmt.Errorf("Max retry but still failure")
)

// RepType present response type.
// classified by
type RepType int

const (
	// OK response success
	OK RepType = 1 << iota
	// Fail response fail
	Fail
	// Breakable indicate can break
	Breakable
	// Retriable indicate can retry
	Retriable
)

// LoadBalancer present lb for special category resources.
// Create LoadBalancer use `NewLoadBalancer`
type LoadBalancer struct {
	servers      nodeList
	distributor  distributor
	metrics      nodeMetrics
	maxRetry     uint
	maxRePick    uint
	repClassify  func(err error) RepType
	retryBackOff BackOff
}

// WithCircuitBreaker configure CircuitBreaker config.
//
// - successiveFailThreshold when successive error more than threshold break will open.
// - trippedTimeFactor opened breaker - -!
// - trippedTimeoutMax indicate maximum tripped timeout growth can reach, default is `Exponentia`
// - trippedBackOff indicate how tripped timeout growth, see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.
func WithCircuitBreaker(successiveFailThreshold, trippedTimeFactor uint, trippedTimeoutMax time.Duration, trippedBackOff BackOff) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.metrics.failureThreshold = successiveFailThreshold
		lb.metrics.trippedTimeoutFactor = trippedTimeFactor
		lb.metrics.trippedTimeoutWindow = trippedTimeoutMax
		lb.metrics.trippedBackOff = trippedBackOff
	}
}

// WithBulkhead configure WithBulkhead config.
//
// - activeReqThreshold indicate maxActiveReqThreshold for one node
// - activeReqCountWindow indicate time window for calculate activeReqCount
func WithBulkhead(activeReqThreshold uint64, activeReqCountWindow time.Duration) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.metrics.activeThreshold = activeReqThreshold
		lb.metrics.activeReqCountWindow = activeReqCountWindow
	}
}

// WithRetry configure Retry config
//
// - maxServerPick indicate maximum retry time for pick other servers.
// - maxRetryPerServe indicate maximum retry on one server
// - retryBackOff indicate backOff between retry interval, default is `DecorrelatedJittered`
// - see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.
func WithRetry(maxServerPick, maxRetryPerServer uint, retryBackOff BackOff) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.maxRePick = maxServerPick
		lb.maxRetry = maxRetryPerServer
		lb.retryBackOff = retryBackOff
	}
}

// WithResponseClassifier config response classification config.
//
// - classifier indicate which classifier use to classify response
//
// Default use `NetworkErrorClassification` which only take care of Golang network error.
func WithResponseClassifier(classifier func(_err error) RepType) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.repClassify = classifier
	}
}

// NewLoadBalancer construct LoadBalancer using given node list
// the node array is provide using string, e.g. `10.10.10.114:9999`
// It's will be tweaked use OptFunction like `WithRetry`, `WithCiruitBreake`, `WithBulkhead`
func NewLoadBalancer(nodes []string, opts ...func(lb *LoadBalancer)) *LoadBalancer {
	servers := make(nodeList, 0, len(nodes))
	for idx, addr := range nodes {
		servers = append(servers, Node{
			index:  idx,
			Server: addr,
		})
	}

	m := newNodeMetric(
		servers,
		DefaultTrippedTimeoutFactor,
		DefaultFailureThreshold,
		DefaultActiveThreshold,
		DefaultTrippedTimeoutMax,
		DefaultActiveReqCountWindow,
	)

	d := newDistributor(nodes, m)

	lb := &LoadBalancer{
		servers:      servers,
		distributor:  *d,
		metrics:      *m,
		maxRetry:     DefaultMaxRetry,
		maxRePick:    DefaultMaxPick,
		repClassify:  NetworkErrorClassification,
		retryBackOff: DecorrelatedJittered,
	}

	for _, opt := range opts {
		opt(lb)
	}

	m.start()

	return lb
}

// Submit submits function which will be triggered on some node to load balancer.
func (l *LoadBalancer) Submit(service func(node *Node) error) error {

	execContext := &executionContext{}

	for execContext.serverAttemptCount <= l.maxRePick {

		execContext.incServerAttemptCount()

		execContext.node = l.distributor.pickServer(&l.metrics, execContext.node, l.availableServer())
		if execContext.node == nil {
			return AllServerDownError
		}

		metric := l.metrics.takeMetric(*execContext.node)
		metric.incActive()
		for execContext.attemptCount <= l.maxRetry {
			execContext.incAttemptCount()
			startTime := time.Now()
			err := service(execContext.node)
			repType := l.repClassify(err)
			rt := time.Now().Sub(startTime)
			switch {
			case repType&OK == OK:
				metric.recordSuccess(rt)
				metric.descActive()
				return nil
			case repType&Breakable == Breakable:
				metric.recordFailure(rt)
			}

			if repType&Retriable != Retriable {
				return err
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

func (l *LoadBalancer) availableServer() nodeList {
	nodes := make([]Node, 0, len(l.servers))
	for _, node := range l.servers {
		if !l.metrics.isCircuitBreakTripped(node) && l.metrics.takeMetric(node).activeReqCount < l.metrics.activeThreshold {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// NetworkErrorClassification uses to classify network error into ok/failure/retriable/breakable
// It's default Response classifier for LoadBalancer.
func NetworkErrorClassification(_err error) RepType {
	var typ RepType
	if _err == nil {
		typ |= OK
		return typ
	}
	typ |= Fail
	switch t := _err.(type) {
	case net.Error:
		if t.Timeout() {
			typ |= Retriable
		}
		typ |= Breakable
		return typ
	case *url.Error:
		if nestErr, ok := t.Err.(net.Error); ok {
			if nestErr.Timeout() {
				typ |= Retriable
			}
			typ |= Breakable
			return typ
		}
	}
	if _err != nil && strings.Contains(_err.Error(), "use of closed network connection") {
		typ |= Breakable
		typ |= Retriable
		return typ
	}
	return typ
}
