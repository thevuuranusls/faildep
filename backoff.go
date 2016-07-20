package faildep

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
type BackOff func(base time.Duration, max time.Duration, attempt uint) time.Duration

// NoBackOff do everything without backOff
func NoBackoff(base time.Duration, max time.Duration, attempt uint) time.Duration {
	return time.Duration(0)
}

// Exponential present exponential-backoff
// via: https://en.wikipedia.org/wiki/Exponential_backoff
func Exponential(base time.Duration, max time.Duration, attempt uint) time.Duration {
	backOff := base * time.Duration((int64(1) << attempt))
	if backOff > max {
		backOff = max
	}
	return backOff
}

// ExponentialJittered present exponential jittered backoff
func ExponentialJittered(base time.Duration, max time.Duration, attempt uint) time.Duration {
	maxBackOff := base * time.Duration((int64(1) << attempt))
	if maxBackOff > max {
		maxBackOff = max
	}
	return time.Duration(rand.Int63n(int64(maxBackOff)))
}

// DecorrelatedJittered present decorrelated jittered backOff
// via: http://www.awsarchitectureblog.com/2015/03/backoff.html
func DecorrelatedJittered(base time.Duration, max time.Duration, attempt uint) time.Duration {
	randRange := base*time.Duration((int64(1)<<attempt)) - base
	randBackOff := base
	if randRange != 0 {
		randBackOff = base + time.Duration(rand.Int63n(int64(randRange)))
	}
	backOff := randBackOff
	if randBackOff > max {
		backOff = max
	}
	return backOff
}
