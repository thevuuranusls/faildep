package slb

type executionContext struct {
	node               *Node
	attemptCount       uint
	serverAttemptCount uint
}

func (c *executionContext) incAttemptCount() {
	c.attemptCount++
}

func (c *executionContext) incServerAttemptCount() {
	c.serverAttemptCount++
}

func (c *executionContext) resetAttemptCount() {
	c.attemptCount = 0
}
