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
	return "error"
}

func TestTryMax_inSingle_nextServers(t *testing.T) {
	lb := NewSLB([]string{"1", "2", "3"}, 1, 4, 4, 3*time.Second, 1*time.Second)
	lb.maxRePick = 2
	lb.maxRetry = 2
	var count int64
	err := lb.Submit(func(node *Node) error {
		atomic.AddInt64(&count, 1)
		return testNetError{}
	})
	assert.Error(t, err)
	assert.Equal(t, int64(9), count)
}

func TestFailureAndRestart(t *testing.T) {
	lb := NewSLB([]string{"1"}, 2, 3, 3, 3*time.Second, 1*time.Second)
	lb.maxRePick = 0
	lb.maxRetry = 0
	for i := 0; i < 3; i++ {
		err := lb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "Max retry but still failure")
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
	lb := NewSLB([]string{"1"}, 2, 3, 3, 10*time.Second, 1*time.Second)
	lb.maxRePick = 0
	lb.maxRetry = 0
	for i := 0; i < 3; i++ {
		err := lb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "Max retry but still failure")
	}

	time.Sleep(2 * time.Second)
	err := lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.Error(t, err)

	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "All Server Has Down")

	time.Sleep(4 * time.Second)
	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "Max retry but still failure")

	time.Sleep(8 * time.Second)
	err = lb.Submit(func(node *Node) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExponentialMaxTime(t *testing.T) {
	lb := NewSLB([]string{"1"}, 2, 3, 3, 10*time.Second, 1*time.Second)
	lb.maxRePick = 0
	lb.maxRetry = 0
	for i := 0; i < 3; i++ {
		err := lb.Submit(func(node *Node) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "Max retry but still failure")
	}

	time.Sleep(2 * time.Second)
	err := lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.Error(t, err)

	time.Sleep(4 * time.Second)
	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "Max retry but still failure")

	time.Sleep(8 * time.Second)
	err = lb.Submit(func(node *Node) error {
		return testNetError{}
	})
	assert.Error(t, err)

	time.Sleep(10 * time.Second)
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
