package slb

import (
	"math/rand"
	"time"
)

func exponential(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration {
	backOff := start * time.Duration((int64(1)<<attempt)*int64(multiplier))
	if backOff > max {
		backOff = max
	}
	return backOff
}

func exponentialJittered(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration {
	maxBackOff := start * time.Duration((int64(1)<<attempt)*int64(multiplier))
	if maxBackOff > max {
		maxBackOff = max
	}
	return time.Duration(rand.Int63n(int64(maxBackOff)))
}

func decorrelatedJittered(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration {
	randRange := start*time.Duration((int64(1)<<attempt)*int64(multiplier)) - start
	randBackOff := start
	if randRange != 0 {
		randBackOff = start + time.Duration(rand.Int63n(int64(randRange)))
	}
	backOff := randBackOff
	if randBackOff > max {
		backOff = max
	}
	return backOff
}
