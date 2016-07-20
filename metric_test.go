package faildep

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMetric_add_and_get(t *testing.T) {

	n := Resource{
		index:  1,
		Server: "123",
	}

	m := newNodeMetric(ResourceList{n})
	m.start()

	nm := m.takeMetric(n)
	nm.recordFailure(1 * time.Millisecond)
	m.takeMetric(n).recordFailure(1 * time.Millisecond)
	mm := m.takeMetric(n)
	assert.Equal(t, uint(2), mm.successiveFailCount)

	m.takeMetric(n).recordSuccess(1 * time.Millisecond)

	mm = m.takeMetric(n)

	assert.Equal(t, uint(0), mm.successiveFailCount)

}
