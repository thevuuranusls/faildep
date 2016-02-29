
# slb
[![GoDoc](https://godoc.org/github.com/lysu/slb?status.svg)](https://godoc.org/github.com/lysu/slb)


    import "github.com/lysu/slb"

slb implements simple client-based LoadBalancer
provide:

- distribute request to nodes in node-list
- circuitBreaker break request when successive error or high concurrent number
- retry in one server or other server


API documentation and examples are available via [godoc](https://godoc.org/github.com/lysu/slb).


## Variables
``` go
var (
    // AllServerDownError returns when all backend server has down
    AllServerDownError = fmt.Errorf("All Server Has Down")
    // MaxRetryError returns when retry beyond given maxRetry time
    MaxRetryError = fmt.Errorf("Max retry but still failure")
)
```

## func DecorrelatedJittered
``` go
func DecorrelatedJittered(base time.Duration, max time.Duration, attempt uint) time.Duration
```
DecorrelatedJittered present decorrelated jittered backOff
via: <a href="http://www.awsarchitectureblog.com/2015/03/backoff.html">http://www.awsarchitectureblog.com/2015/03/backoff.html</a>


## func Exponential
``` go
func Exponential(base time.Duration, max time.Duration, attempt uint) time.Duration
```
Exponential present exponential-backoff
via: <a href="https://en.wikipedia.org/wiki/Exponential_backoff">https://en.wikipedia.org/wiki/Exponential_backoff</a>


## func ExponentialJittered
``` go
func ExponentialJittered(base time.Duration, max time.Duration, attempt uint) time.Duration
```
ExponentialJittered present exponential jittered backoff


## func NoBackoff
``` go
func NoBackoff(base time.Duration, max time.Duration, attempt uint) time.Duration
```
NoBackOff do everything without backOff


## func WithBulkhead
``` go
func WithBulkhead(activeReqThreshold uint64, activeReqCountWindow time.Duration) func(lb *LoadBalancer)
```
WithBulkhead configure WithBulkhead config.

Default: Bulkhead is disabled, we must use this OptFunc to enable it.

- activeReqThreshold indicate maxActiveReqThreshold for one node
- activeReqCountWindow indicate time window for calculate activeReqCount


## func WithCircuitBreaker
``` go
func WithCircuitBreaker(successiveFailThreshold uint, trippedBaseTime time.Duration, trippedTimeoutMax time.Duration, trippedBackOff BackOff) func(lb *LoadBalancer)
```
WithCircuitBreaker configure CircuitBreaker config.

Default: circuitBreaker is disabled, we must use this OptFunc to enable it.

- successiveFailThreshold when successive error more than threshold break will open.
- trippedBaseTime indicate first trip time when breaker open, and successive error will increase base on it.
- trippedTimeoutMax indicate maximum tripped time after growth when successive error occur
- trippedBackOff indicate how tripped timeout growth, see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.


## func WithPickServer
``` go
func WithPickServer(sp PickServer) func(lb *LoadBalancer)
```
WithPickServer config server pick logic.
Default use `P2CPick` to pick server.


## func WithResponseClassifier
``` go
func WithResponseClassifier(classifier func(_err error) RepType) func(lb *LoadBalancer)
```
WithResponseClassifier config response classification config.

- classifier indicate which classifier use to classify response

Default use `NetworkErrorClassification` which only take care of Golang network error.


## func WithRetry
``` go
func WithRetry(maxServerPick, maxRetryPerServer uint, retryBaseInterval, retryMaxInterval time.Duration, retryBackOff BackOff) func(lb *LoadBalancer)
```
WithRetry configure Retry config

Default: Retry is disabled, we must use this OptFunc to enable it.

- maxServerPick indicate maximum retry time for pick other servers.
- maxRetryPerServe indicate maximum retry on one server
- retryBaseInterval indicate first retry time interval and continue action will base on it.
- retryMaxInterval indicate maximum retry interval after successive error.
- retryBackOff indicate backOff between retry interval, default is `DecorrelatedJittered`
- see backoff.go: `Exponential`, `ExponentialJittered`, `DecorrelatedJittered`.



## type BackOff
``` go
type BackOff func(base time.Duration, max time.Duration, attempt uint) time.Duration
```
Backoff present strategy for how backOff when something be called many times.

- start: first backoff
- multipler:
- max: maximum backOff
- attemp: current attemp count











## type LoadBalancer
``` go
type LoadBalancer struct {
    // contains filtered or unexported fields
}
```
LoadBalancer present lb for special category resources.
Create LoadBalancer use `NewLoadBalancer`









### func NewLoadBalancer
``` go
func NewLoadBalancer(nodes []string, opts ...func(lb *LoadBalancer)) *LoadBalancer
```
NewLoadBalancer construct LoadBalancer using given node list
the node array is provide using string, e.g. `10.10.10.114:9999`
It's will be tweaked use OptFunction like `WithRetry`, `WithCiruitBreake`, `WithBulkhead`




### func (\*LoadBalancer) Submit
``` go
func (l *LoadBalancer) Submit(service func(node *Node) error) error
```
Submit submits function which will be triggered on some node to load balancer.



## type Node
``` go
type Node struct {

    // Server present server name.
    // e.g. 0.0.0.0:9999
    Server string
    // contains filtered or unexported fields
}
```
Node present a resource.









### func P2CPick
``` go
func P2CPick(metrics *nodeMetrics, currentServer *Node, allNodes NodeList) *Node
```
P2CPick picks server using P2C
<a href="https://www.eecs.harvard.edu/~michaelm/postscripts/tpds2001.pdf">https://www.eecs.harvard.edu/~michaelm/postscripts/tpds2001.pdf</a>


### func RandomPick
``` go
func RandomPick(metrics *nodeMetrics, currentServer *Node, servers NodeList) *Node
```
RandomPick picks server using random index.




## type NodeList
``` go
type NodeList []Node
```
NodeList present resource node list.











## type PickServer
``` go
type PickServer func(metrics *nodeMetrics, currentServer *Node, servers NodeList) *Node
```
PickServer present pick server logic.
NewPick server logic must use this contract.











## type RepType
``` go
type RepType int
```
RepType present response type.
classified by



``` go
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
```






### func NetworkErrorClassification
``` go
func NetworkErrorClassification(_err error) RepType
```
NetworkErrorClassification uses to classify network error into ok/failure/retriable/breakable
It's default Response classifier for LoadBalancer.










- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
