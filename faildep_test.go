package faildep

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

// This example show how to initialize FailDep with `retry`, `bulkheads` and `circuitBreaker`
// and execute a operation on server.
func ExampleFaildep() {
	faildep := NewFailDep(
		"exampleDep",
		[]string{"server1:80", "server2:80"},                                             // 2 backend nodes
		WithRetry(5, 1, 30*time.Millisecond, 100*time.Millisecond, DecorrelatedJittered), // retry 1 time on old server and max repick server 5 times, otherwise break.
		WithBulkhead(10, 1*time.Second),                                                  // in 1 second window, max 10 request on running, , otherwise break.
		WithCircuitBreaker(5, 2*time.Millisecond, 500*time.Millisecond, Exponential),     // max success 5 times, and exponential break timeout.
		WithResponseClassifier(NetworkErrorClassification),
	)
	err := faildep.Do(func(node *Resource) error { // function will call for special node for many times when retry.
		err := doSomeOneNode(node) // do some on node, like issue Request, send data..
		return err                 // err indicate whether operation execution result, and it will be classified by `ResponseClassifier`
	})
	if err != nil {
		// recheck error return by f.
	}
}

func doSomeOneNode(node *Resource) error {
	return nil
}

func TestTryMax_inSingle_nextServers(t *testing.T) {
	f := NewFailDep("testTryMax", []string{"1", "2", "3"},
		WithRetry(2, 2, 20*time.Millisecond, 100*time.Millisecond, DecorrelatedJittered),
	)
	var count int64
	err := f.Do(func(node *Resource) error {
		atomic.AddInt64(&count, 1)
		return testNetError{}
	})
	assert.Error(t, err)
	assert.Equal(t, int64(9), count)
}

func TestDisableBreaker(t *testing.T) {

	f := NewFailDep("testDisable", []string{"1"})
	for i := 0; i < 3; i++ {
		err := f.Do(func(node *Resource) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}
	err := f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "realError")

	f2 := NewFailDep("testDisable2", []string{"1"}, WithCircuitBreaker(3, 2*time.Millisecond, 3*time.Second, Exponential))
	for i := 0; i < 3; i++ {
		err := f2.Do(func(node *Resource) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}
	err = f2.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "All Resource Has Down")

}

func TestFailureAndRestart(t *testing.T) {
	f := NewFailDep("testFailRestart", []string{"1"},
		WithCircuitBreaker(3, 2*time.Millisecond, 3*time.Second, Exponential),
	)
	for i := 0; i < 3; i++ {
		err := f.Do(func(node *Resource) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}
	err := f.Do(func(node *Resource) error {
		return nil
	})
	assert.EqualError(t, err, "All Resource Has Down")
	time.Sleep(2 * time.Second)
	err = f.Do(func(node *Resource) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExponentialTrippedTime(t *testing.T) {
	f := NewFailDep("testExp", []string{"1"},
		WithCircuitBreaker(3, 2*time.Millisecond, 10*time.Second, Exponential),
	)
	for i := 0; i < 3; i++ {
		err := f.Do(func(node *Resource) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}

	time.Sleep(1 * time.Millisecond)
	err := f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.Error(t, err)

	err = f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "All Resource Has Down")

	time.Sleep(2 * time.Millisecond)
	err = f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "realError")

	time.Sleep(1 * time.Millisecond)
	err = f.Do(func(node *Resource) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExponentialMaxTime(t *testing.T) {
	f := NewFailDep("testExp", []string{"1"},
		WithCircuitBreaker(3, 2*time.Millisecond, 10*time.Second, Exponential),
	)
	for i := 0; i < 3; i++ {
		err := f.Do(func(node *Resource) error {
			return testNetError{}
		})
		assert.EqualError(t, err, "realError")
	}

	time.Sleep(2 * time.Millisecond)
	err := f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.Error(t, err)

	time.Sleep(4 * time.Millisecond)
	err = f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.EqualError(t, err, "realError")

	time.Sleep(8 * time.Millisecond)
	err = f.Do(func(node *Resource) error {
		return testNetError{}
	})
	assert.Error(t, err)

	time.Sleep(10 * time.Millisecond)
	err = f.Do(func(node *Resource) error {
		return nil
	})
	assert.NoError(t, err)
}
