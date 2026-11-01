package main

import (
	"sync"
	"time"
)

type Clock interface{ Now() time.Time }
type Limiter struct {
	mu     sync.Mutex
	clock  Clock
	rate   float64
	burst  int
	tokens float64
	last   time.Time
}

func NewLimiter(clock Clock, ratePerSec float64, burst int) *Limiter {
	l := &Limiter{clock: clock, rate: max(ratePerSec, 0), burst: burst}
	if clock == nil || burst <= 0 {
		return l
	}
	l.tokens = float64(burst)
	l.last = clock.Now()
	return l
}

func (l *Limiter) AllowN(n int) bool {
	if l.clock == nil || l.burst <= 0 || n <= 0 || n > l.burst {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill(l.clock.Now())

	if l.tokens < float64(n) {
		return false
	}
	l.tokens -= float64(n)
	return true
}

func (l *Limiter) refill(now time.Time) {
	if !now.After(l.last) {
		return
	}
	elapsed := now.Sub(l.last).Seconds()
	l.tokens = min(l.tokens+elapsed*l.rate, float64(l.burst))
	l.last = now
}
