package auth

import (
	"sync"
	"time"
)

// rateLimiter is a simple per-key sliding-window limiter used to cap guest code
// attempts per IP (the 4-char code space is small enough to brute-force).
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
	now    func() time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		hits:   make(map[string][]time.Time),
		max:    max,
		window: window,
		now:    time.Now,
	}
}

// Allow records an attempt for key and reports whether it is within the limit.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	cutoff := now.Add(-rl.window)

	// Drop timestamps outside the window.
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= rl.max {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}
