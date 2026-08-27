package auth

import (
	"testing"
	"time"
)

func TestRateLimiterCapsAttempts(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := range 3 {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("4th attempt should be denied")
	}
}

func TestRateLimiterIsPerKey(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	if !rl.Allow("a") {
		t.Fatal("first key-a attempt should be allowed")
	}
	if !rl.Allow("b") {
		t.Fatal("first key-b attempt should be allowed (separate key)")
	}
	if rl.Allow("a") {
		t.Fatal("second key-a attempt should be denied")
	}
}

func TestRateLimiterWindowExpires(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(1, time.Minute)
	rl.now = func() time.Time { return now }

	if !rl.Allow("k") {
		t.Fatal("first attempt should be allowed")
	}
	if rl.Allow("k") {
		t.Fatal("second attempt in-window should be denied")
	}
	now = now.Add(2 * time.Minute) // slide past the window
	if !rl.Allow("k") {
		t.Fatal("attempt after window should be allowed again")
	}
}
