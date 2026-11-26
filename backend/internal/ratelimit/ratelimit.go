// Package ratelimit provides a small in-memory fixed-window rate limiter.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most limit events per window for each key.
type Limiter[K comparable] struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu        sync.Mutex
	counters  map[K]counter
	lastSweep time.Time
}

type counter struct {
	windowEnd time.Time
	count     int
}

// New returns a Limiter allowing limit events per window for each key.
func New[K comparable](limit int, window time.Duration) *Limiter[K] {
	return newWithClock[K](limit, window, time.Now)
}

func newWithClock[K comparable](limit int, window time.Duration, now func() time.Time) *Limiter[K] {
	return &Limiter[K]{limit: limit, window: window, now: now, counters: map[K]counter{}}
}

// Allow records one event for key. When the quota is exhausted it returns
// false together with the time left until the quota is renewed.
func (l *Limiter[K]) Allow(key K) (ok bool, retryAfter time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	c := l.counters[key]
	if !now.Before(c.windowEnd) {
		c = counter{windowEnd: now.Add(l.window)}
	}
	if c.count >= l.limit {
		return false, c.windowEnd.Sub(now)
	}
	c.count++
	l.counters[key] = c
	return true, 0
}

// sweep drops expired counters so the map cannot grow without bound. It runs
// at most once per window. The caller must hold l.mu.
func (l *Limiter[K]) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for k, c := range l.counters {
		if !now.Before(c.windowEnd) {
			delete(l.counters, k)
		}
	}
}
