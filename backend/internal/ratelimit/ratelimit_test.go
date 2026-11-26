package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newWithClock[int64](2, time.Minute, func() time.Time { return now })

	for i := range 2 {
		if ok, _ := l.Allow(1); !ok {
			t.Fatalf("event %d must be allowed", i)
		}
	}
	now = now.Add(20 * time.Second)
	ok, retry := l.Allow(1)
	if ok || retry != 40*time.Second {
		t.Fatalf("ok=%v retry=%v, want denied with 40s", ok, retry)
	}
	if ok, _ := l.Allow(2); !ok {
		t.Fatal("keys must be independent")
	}

	now = now.Add(41 * time.Second)
	if ok, _ := l.Allow(1); !ok {
		t.Fatal("quota must renew after the window")
	}
}

func TestLimiterSweepsExpiredKeys(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	l := newWithClock[int](1, time.Minute, func() time.Time { return now })
	for k := range 100 {
		l.Allow(k)
	}
	now = now.Add(2 * time.Minute)
	l.Allow(1000)
	if len(l.counters) != 1 {
		t.Fatalf("counters = %d, want 1", len(l.counters))
	}
}
