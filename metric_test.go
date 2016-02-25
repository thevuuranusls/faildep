package slb

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMetric_add_and_get(t *testing.T) {

	n := Node{
		index:  1,
		server: "123",
	}

	m := newNodeMetric(nodeList{n}, 10, 10, 10, 10*time.Second, 10*time.Second)

	nm := m.takeMetric(n)
	nm.recordFailure(1 * time.Millisecond)
	m.takeMetric(n).recordFailure(1 * time.Millisecond)
	mm := m.takeMetric(n)
	assert.Equal(t, uint(2), mm.successiveFailCount)

	m.takeMetric(n).recordSuccess(1 * time.Millisecond)

	mm = m.takeMetric(n)

	assert.Equal(t, uint(0), mm.successiveFailCount)

}
