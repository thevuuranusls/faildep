package slb

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestExpBackoff(t *testing.T) {
	x := exponential(1*time.Second, 2, 10*time.Second, 0)
	assert.Equal(t, 2*time.Second, x)
	x = exponential(1*time.Second, 2, 10*time.Second, 1)
	assert.Equal(t, 4*time.Second, x)
	x = exponential(1*time.Second, 2, 10*time.Second, 2)
	assert.Equal(t, 8*time.Second, x)
	x = exponential(1*time.Second, 2, 20*time.Second, 3)
	assert.Equal(t, 16*time.Second, x)
	x = exponential(1*time.Second, 2, 20*time.Second, 4)
	assert.Equal(t, 20*time.Second, x)
}

func TestDecorrelatedJettered(t *testing.T) {

	x := decorrelatedJittered(1*time.Second, 2, 10*time.Second, 0)
	fmt.Println(x)
	x = decorrelatedJittered(1*time.Second, 2, 10*time.Second, 1)
	fmt.Println(x)
	x = decorrelatedJittered(1*time.Second, 2, 10*time.Second, 2)
	fmt.Println(x)
	x = decorrelatedJittered(1*time.Second, 2, 10*time.Second, 3)
	fmt.Println(x)

}
