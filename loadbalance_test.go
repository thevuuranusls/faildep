package slb

import (
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
	"time"
)

type testNetError struct {
}

func (t testNetError) Timeout() bool {
	return true
}

func (t testNetError) Temporary() bool {
	return true
}

func (t testNetError) Error() string {
	return "realError"
}

// This example show how to initialize LoadBalancer with `retry`, `bulkheads` and `circuitBreaker`
// and execute a operation on server.
func ExampleLoadBalancer_transfer() {
	lb := NewLoadBalancer(
		[]string{"server1:80", "server2:80"},                                             // 2 backend nodes
		WithRetry(5, 1, 30*time.Millisecond, 100*time.Millisecond, DecorrelatedJittered), // retry 1 time on old server and max repick server 5 times, otherwise break.
		WithBulkhead(10, 1*time.Second),                                                  // in 1 second window, max 10 request on running, , otherwise break.
		WithCircuitBreaker(5, 2*time.Millisecond, 500*time.Millisecond, Exponential),     // max success 5 times, and exponential break timeout.
		WithResponseClassifier(NetworkErrorClassification),
	)
	err := lb.Submit(func(node *Node) error { // function will call for special node for many times when retry.
		err := doSomeOneNode(node) // do some on node, like issue Request, send data..
		return err                 // err indicate whether operation execution result, and it will be classified by `ResponseClassifier`
	})
	if err != nil {
		// recheck error return by lb.
	}
}

func doSomeOneNode(node *Node) error {
	return nil
}

func TestTryMax_inSingle_nextServers(t *testing.T) {
	lb := NewLoadBalancer([]string{"1", "2", "3"},
		WithRetry(2, 2, 20*time.Millisecond, 100*time.Millisecond, DecorrelatedJittered),
	)
	var count int64
	err := lb.Submit(func(node *Node) error {
		atomic.AddInt64(&count, 1)
		return testNetError{}
	})
	assert.Error(t, err)
	assert.Equal(t, int64(9), count)
}

func TestDisableBreaker(t *testing.T) {

	dlb := NewLoadBalancer([]string{"1"})
	for i := 0; i < 3; i++ {
		err := dlb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}
	err := dlb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "realError")

	clb := NewLoadBalancer([]string{"1"}, WithCircuitBreaker(3, 2*time.Millisecond, 3*time.Second, Exponential))
	for i := 0; i < 3; i++ {
		err := clb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}
	err = clb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "All Server Has Down")

}

func TestFailureAndRestart(t *testing.T) {
	lb := NewLoadBalancer([]string{"1"},
		WithCircuitBreaker(3, 2*time.Millisecond, 3*time.Second, Exponential),
	)
	for i := 0; i < 3; i++ {
		err := lb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}
	err := lb.Submit(func(node *Node) error {
		return nil
	})
	assert.EqualError(t, err, "All Server Has Down")
	time.Sleep(2 * time.Second)
	err = lb.Submit(func(node *Node) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExponentialTrippedTime(t *testing.T) {
	lb := NewLoadBalancer([]string{"1"},
		WithCircuitBreaker(3, 2*time.Millisecond, 10*time.Second, Exponential),
	)
	for i := 0; i < 3; i++ {
		err := lb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}

	time.Sleep(1 * time.Millisecond)
	err := lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.Error(t, err)

	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "All Server Has Down")

	time.Sleep(2 * time.Millisecond)
	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "realError")

	time.Sleep(1 * time.Millisecond)
	err = lb.Submit(func(node *Node) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExponentialMaxTime(t *testing.T) {
	lb := NewLoadBalancer([]string{"1"},
		WithCircuitBreaker(3, 2*time.Millisecond, 10*time.Second, Exponential),
	)
	for i := 0; i < 3; i++ {
		err := lb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}

	time.Sleep(2 * time.Millisecond)
	err := lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.Error(t, err)

	time.Sleep(4 * time.Millisecond)
	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "realError")

	time.Sleep(8 * time.Millisecond)
	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.Error(t, err)

	time.Sleep(10 * time.Millisecond)
	err = lb.Submit(func(node *Node) error {
		return nil
	})
	assert.NoError(t, err)
}

//func TestSlowConcurrent(t *testing.T) {
//	lb := NewLoadBalancer([]string{"a", "b"}, 2, 3, 3, 10*time.Second, 1*time.Second)
//	for i := 0; i < 7; i++ {
//		go func() {
//			for {
//				err := lb.Submit(func(node *node) error {
//					time.Sleep(2 * time.Second)
//					return nil
//				})
//				if err != nil {
//					fmt.Println("[Result]----->", err)
//				}
//			}
//		}()
//	}
//}
