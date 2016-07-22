// faildep implements common dependence resource failure handling as a basic library.
// provide:
// - dispatch request to available resource in resource list
// - circuitBreaker break request when successive error or high concurrent number
// - retry in one resource or try to do it in other resources
package faildep

import (
	"encoding/json"
	"fmt"
	"github.com/faildep/faildep-log"
	"net"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

var (
	// AllResourceDownError returns when all backend resource has down
	AllResourceDownError = fmt.Errorf("All Resource Has Down")
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
	circuitBreaker funcFlag = 1 << iota
	bulkhead
	retry
)

// Faildep present failable resources.
// Create Faildep use `NewFaildep`
type FailDep struct {
	name              string
	funcFlags         funcFlag
	servers           ResourceList
	distributor       dispatcher
	metrics           resourceMetrics
	maxRetry          uint
	maxRePick         uint
	repClassify       func(err error) RepType
	retryBaseInterval time.Duration
	retryMaxInterval  time.Duration
	retryBackOff      BackOff
	logger            log.Logger
}

// WithCircuitBreaker configure CircuitBreaker config.
//
// Default: circuitBreaker is disabled, we must use this OptFunc to enable it.
//
// - successiveFailThreshold when successive error more than threshold break will open.
// - trippedBaseTime indicate first trip time when breaker open, and successive error will increase base on it.
// - trippedTimeoutMax indicate maximum tripped time after growth when successive error occur
// - trippedBackOff indicate how tripped timeout growth, see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.
func WithCircuitBreaker(successiveFailThreshold uint, trippedBaseTime time.Duration, trippedTimeoutMax time.Duration, trippedBackOff BackOff) func(f *FailDep) {
	return func(f *FailDep) {
		f.funcFlags |= circuitBreaker
		f.metrics.failureThreshold = successiveFailThreshold
		f.metrics.trippedBaseTime = trippedBaseTime
		f.metrics.trippedTimeoutMax = trippedTimeoutMax
		f.metrics.trippedBackOff = trippedBackOff
	}
}

// WithBulkhead configure WithBulkhead config.
//
// Default: Bulkhead is disabled, we must use this OptFunc to enable it.
//
// - activeReqThreshold indicate maxActiveReqThreshold for one node
// - activeReqCountWindow indicate time window for calculate activeReqCount
func WithBulkhead(activeReqThreshold uint64, activeReqCountWindow time.Duration) func(f *FailDep) {
	return func(f *FailDep) {
		f.funcFlags |= bulkhead
		f.metrics.activeThreshold = activeReqThreshold
		f.metrics.activeReqCountWindow = activeReqCountWindow
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
func WithRetry(maxServerPick, maxRetryPerServer uint, retryBaseInterval, retryMaxInterval time.Duration, retryBackOff BackOff) func(f *FailDep) {
	return func(f *FailDep) {
		f.funcFlags |= retry
		f.maxRePick = maxServerPick
		f.maxRetry = maxRetryPerServer
		f.retryBackOff = retryBackOff
		f.retryBaseInterval = retryBaseInterval
		f.retryMaxInterval = retryMaxInterval
	}
}

// WithResponseClassifier config response classification config.
//
// - classifier indicate which classifier use to classify response
//
// Default use `NetworkErrorClassification` which only take care of Golang network error.
func WithResponseClassifier(classifier func(_err error) RepType) func(f *FailDep) {
	return func(f *FailDep) {
		f.repClassify = classifier
	}
}

// WithPickServer config server pick logic.
// Default use `P2CPick` to pick server.
func WithPickServer(sp ServerPicker) func(f *FailDep) {
	return func(f *FailDep) {
		f.distributor.srvPicker = sp
	}
}

// NewFailDep construct FailDep using given node list
// the node array is provide using string, e.g. `10.10.10.10:9999`
// It's will be tweaked use OptFunction like `WithRetry`, `WithCiruitBreake`, `WithBulkhead`
func NewFailDep(name string, nodes []string, opts ...func(f *FailDep)) *FailDep {
	servers := make(ResourceList, 0, len(nodes))
	for idx, addr := range nodes {
		servers = append(servers, Resource{
			index:  idx,
			Server: addr,
		})
	}

	m := newNodeMetric(servers)

	d := newDispatcher(nodes, m)

	f := &FailDep{
		name:         name,
		funcFlags:    0,
		servers:      servers,
		distributor:  *d,
		metrics:      *m,
		repClassify:  NetworkErrorClassification,
		retryBackOff: DecorrelatedJittered,
		logger:       &log.StdLogger{},
	}

	for _, opt := range opts {
		opt(f)
	}

	m.start()

	return f
}

// Do execute function which will be triggered on some node to do something.
func (f *FailDep) Do(service func(node *Resource) error) error {

	execContext := &executionContext{}

	for execContext.serverAttemptCount <= f.maxRePick {

		execContext.incServerAttemptCount()

		execContext.node = f.distributor.srvPicker(&f.metrics, execContext.node, f.availableServer())
		if execContext.node == nil {
			f.logger.Error("res:", f.name, "s-attempt:", execContext.serverAttemptCount, "error:", "AllServerHasDown")
			return AllResourceDownError
		}

		metric := f.metrics.takeMetric(*execContext.node)
		finish, err := func() (finish bool, errorOut error) {
			metric.incActive()
			defer metric.descActive()
			for execContext.attemptCount <= f.maxRetry {
				execContext.incAttemptCount()
				startTime := time.Now()
				err := service(execContext.node)
				if err != nil {
					f.logger.Warning("res:", f.name, "s-attempt:", execContext.serverAttemptCount,
						"at:", execContext.node.Server, "r-attempt:", execContext.attemptCount,
						"error:", err,
					)
				}
				repType := f.repClassify(err)
				rt := time.Now().Sub(startTime)
				f.logger.Info("res:", f.name, "used:", rt.Nanoseconds()/1000,
					"at:", execContext.node.Server)
				switch {
				case repType&OK == OK:
					metric.recordSuccess(rt)
					finish = true
					return
				case repType&Breakable == Breakable:
					metric.recordFailure(rt)
				}

				if f.funcFlags&retry != retry || repType&Retriable != Retriable {
					finish = true
					errorOut = err
					return
				}

				backOffTime := f.retryBackOff(f.retryBaseInterval, f.retryMaxInterval, execContext.attemptCount)
				if backOffTime > 0 {
					time.Sleep(backOffTime)
				}
			}
			execContext.resetAttemptCount()
			finish = false
			return
		}()
		if finish {
			if err == nil {
				f.logger.Error("res:", f.name, "s-attempt:", execContext.serverAttemptCount, "error:", err)
			}
			return err
		}
	}

	return MaxRetryError
}

func (f *FailDep) availableServer() ResourceList {
	nodes := make([]Resource, 0, len(f.servers))
	for _, node := range f.servers {
		if !(f.funcFlags&circuitBreaker == circuitBreaker && f.metrics.isCircuitBreakTripped(node)) &&
			!(f.funcFlags&bulkhead == bulkhead && f.metrics.takeMetric(node).activeReqCount >= f.metrics.activeThreshold) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

type stats struct {
	Av        bool   `json:"av"`
	Srv       string `json:"srv"`
	ActiveReq uint64 `json:"activeReq"`
	FailCount uint   `json:"failCount"`
}

func (f *FailDep) logStats() {
	defer func() {
		if r := recover(); r != nil {
			f.logger.Error("Panic Occured:", r, string(debug.Stack()))
		}
	}()
	for range time.Tick(1 * time.Second) {
		ss := make([]stats, len(f.servers))
		for _, node := range f.servers {
			av := false
			metric := f.metrics.takeMetric(node)
			if !(f.funcFlags&circuitBreaker == circuitBreaker && f.metrics.isCircuitBreakTripped(node)) &&
				!(f.funcFlags&bulkhead == bulkhead && metric.takeActiveReqCount() >= f.metrics.activeThreshold) {
				av = true
			}
			ss = append(ss, stats{
				Av:        av,
				Srv:       node.Server,
				FailCount: metric.takeFailCount(),
				ActiveReq: metric.takeActiveReqCount(),
			})
		}

		for _, s := range ss {
			statsJSON, err := json.Marshal(s)
			if err != nil {
				continue
			}
			f.logger.Info("res:", f.name, "statu:", string(statsJSON))
		}

	}
}

// NetworkErrorClassification uses to classify network error into ok/failure/retriable/breakable
// It's default Response classifier for FailDep.
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
		if strings.Contains(_err.Error(), "connection refused") {
			typ |= Retriable
		}
		typ |= Breakable
		return typ
	case *url.Error:
		if nestErr, ok := t.Err.(net.Error); ok {
			if nestErr.Timeout() {
				typ |= Retriable
			}
			if strings.Contains(_err.Error(), "connection refused") {
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
