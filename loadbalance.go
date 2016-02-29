package slb

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
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

type funcFlag int

const (
	circuitBreaker = 1 << iota
	bulkhead
	retry
)

// LoadBalancer present lb for special category resources.
// Create LoadBalancer use `NewLoadBalancer`
type LoadBalancer struct {
	funcFlags         funcFlag
	servers           NodeList
	distributor       distributor
	metrics           nodeMetrics
	maxRetry          uint
	maxRePick         uint
	repClassify       func(err error) RepType
	retryBaseInterval time.Duration
	retryMaxInterval  time.Duration
	retryBackOff      BackOff
}

// WithCircuitBreaker configure CircuitBreaker config.
//
// Default: circuitBreaker is disabled, we must use this OptFunc to enable it.
//
// - successiveFailThreshold when successive error more than threshold break will open.
// - trippedBaseTime indicate first trip time when breaker open, and successive error will increase base on it.
// - trippedTimeoutMax indicate maximum tripped time after growth when successive error occur
// - trippedBackOff indicate how tripped timeout growth, see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.
func WithCircuitBreaker(successiveFailThreshold uint, trippedBaseTime time.Duration, trippedTimeoutMax time.Duration, trippedBackOff BackOff) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.funcFlags |= circuitBreaker
		lb.metrics.failureThreshold = successiveFailThreshold
		lb.metrics.trippedBaseTime = trippedBaseTime
		lb.metrics.trippedTimeoutMax = trippedTimeoutMax
		lb.metrics.trippedBackOff = trippedBackOff
	}
}

// WithBulkhead configure WithBulkhead config.
//
// Default: Bulkhead is disabled, we must use this OptFunc to enable it.
//
// - activeReqThreshold indicate maxActiveReqThreshold for one node
// - activeReqCountWindow indicate time window for calculate activeReqCount
func WithBulkhead(activeReqThreshold uint64, activeReqCountWindow time.Duration) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.funcFlags |= bulkhead
		lb.metrics.activeThreshold = activeReqThreshold
		lb.metrics.activeReqCountWindow = activeReqCountWindow
	}
}

// WithRetry configure Retry config
//
// Default: Retry is disabled, we must use this OptFunc to enable it.
//
// - maxServerPick indicate maximum retry time for pick other servers.
// - maxRetryPerServe indicate maximum retry on one server
// - retryBaseInterval indicate first retry time interval and continue action will base on it.
// - retryMaxInterval indicate maximum retry interval after successive error.
// - retryBackOff indicate backOff between retry interval, default is `DecorrelatedJittered`
// - see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.
func WithRetry(maxServerPick, maxRetryPerServer uint, retryBaseInterval, retryMaxInterval time.Duration, retryBackOff BackOff) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.funcFlags |= retry
		lb.maxRePick = maxServerPick
		lb.maxRetry = maxRetryPerServer
		lb.retryBackOff = retryBackOff
		lb.retryBaseInterval = retryBaseInterval
		lb.retryMaxInterval = retryMaxInterval
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

// WithPickServer config server pick logic.
// Default use `P2CPick` to pick server.
func WithPickServer(sp PickServer) func(lb *LoadBalancer) {
	return func(lb *LoadBalancer) {
		lb.distributor.pickServer = sp
	}
}

// NewLoadBalancer construct LoadBalancer using given node list
// the node array is provide using string, e.g. `10.10.10.114:9999`
// It's will be tweaked use OptFunction like `WithRetry`, `WithCiruitBreake`, `WithBulkhead`
func NewLoadBalancer(nodes []string, opts ...func(lb *LoadBalancer)) *LoadBalancer {
	servers := make(NodeList, 0, len(nodes))
	for idx, addr := range nodes {
		servers = append(servers, Node{
			index:  idx,
			Server: addr,
		})
	}

	m := newNodeMetric(servers)

	d := newDistributor(nodes, m)

	lb := &LoadBalancer{
		funcFlags:    0,
		servers:      servers,
		distributor:  *d,
		metrics:      *m,
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
		finish, err := func() (finish bool, errorOut error) {
			metric.incActive()
			defer metric.descActive()
			for execContext.attemptCount <= l.maxRetry {
				execContext.incAttemptCount()
				startTime := time.Now()
				err := service(execContext.node)
				repType := l.repClassify(err)
				rt := time.Now().Sub(startTime)
				switch {
				case repType&OK == OK:
					metric.recordSuccess(rt)
					finish = true
					return
				case repType&Breakable == Breakable:
					metric.recordFailure(rt)
				}

				if l.funcFlags&retry != retry || repType&Retriable != Retriable {
					finish = true
					errorOut = err
					return
				}

				backOffTime := l.retryBackOff(l.retryBaseInterval, l.retryMaxInterval, execContext.attemptCount)
				if backOffTime > 0 {
					time.Sleep(backOffTime)
				}
			}
			execContext.resetAttemptCount()
			finish = false
			return
		}()
		if finish {
			return err
		}
	}

	return MaxRetryError
}

func (l *LoadBalancer) availableServer() NodeList {
	nodes := make([]Node, 0, len(l.servers))
	for _, node := range l.servers {
		if !(l.funcFlags&circuitBreaker == circuitBreaker && l.metrics.isCircuitBreakTripped(node)) &&
			!(l.funcFlags&bulkhead == bulkhead && l.metrics.takeMetric(node).activeReqCount >= l.metrics.activeThreshold) {
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
