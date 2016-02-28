package slb

import (
	"math/rand"
	"time"
)

// Backoff present strategy for how backOff when something be called many times.
//
// - start: first backoff
// - multipler:
// - max: maximum backOff
// - attemp: current attemp count
type BackOff func(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration

// Exponential present exponential-backoff
// via: https://en.wikipedia.org/wiki/Exponential_backoff
func Exponential(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration {
	backOff := start * time.Duration((int64(1)<<attempt)*int64(multiplier))
	if backOff > max {
		backOff = max
	}
	return backOff
}

// ExponentialJittered present exponential jittered backoff
func ExponentialJittered(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration {
	maxBackOff := start * time.Duration((int64(1)<<attempt)*int64(multiplier))
	if maxBackOff > max {
		maxBackOff = max
	}
	return time.Duration(rand.Int63n(int64(maxBackOff)))
}

// DecorrelatedJittered present decorrelated jittered backOff
// via: http://www.awsarchitectureblog.com/2015/03/backoff.html
func DecorrelatedJittered(start time.Duration, multiplier uint, max time.Duration, attempt uint) time.Duration {
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
