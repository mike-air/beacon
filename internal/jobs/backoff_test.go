// Unit test for the retry backoff — pure logic, no database.
//
// Course mapping: Chapter 38 — unit tests. Backoff must be monotonic up to its
// cap and never exceed 5 minutes, so a failing job reschedules sanely.
package jobs

import (
	"testing"
	"time"
)

// maxBackoff is the largest value Backoff can return: a true 5-minute ceiling.
const maxBackoff = 5 * time.Minute

func TestBackoffValues(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{8, 256 * time.Second}, // 2^8 = 256s, still under the ceiling
		{9, maxBackoff},        // 2^9 = 512s → clamped to the 5-minute ceiling
		{20, maxBackoff},       // far past the clamp; still the ceiling
	}
	for _, tt := range tests {
		if got := Backoff(tt.attempt); got != tt.want {
			t.Errorf("Backoff(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestBackoffMonotonicAndCapped(t *testing.T) {
	var prev time.Duration
	for attempt := 1; attempt <= 30; attempt++ {
		d := Backoff(attempt)
		if d < prev {
			t.Fatalf("Backoff not monotonic: Backoff(%d)=%v < previous %v", attempt, d, prev)
		}
		if d > maxBackoff {
			t.Fatalf("Backoff(%d)=%v exceeds the 5-minute ceiling", attempt, d)
		}
		prev = d
	}
	if prev != maxBackoff {
		t.Errorf("Backoff never reached its ceiling; last = %v, want %v", prev, maxBackoff)
	}
}
