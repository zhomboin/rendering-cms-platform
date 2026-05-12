package comments

import (
	"testing"
	"time"
)

func TestAllowCommentWithinLimit(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	recent := []time.Time{
		now.Add(-10 * time.Second),
		now.Add(-30 * time.Second),
	}
	if !AllowComment(now, recent) {
		t.Fatal("expected comment to be allowed")
	}
}

func TestRejectCommentOverLimit(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	recent := []time.Time{
		now.Add(-10 * time.Second),
		now.Add(-20 * time.Second),
		now.Add(-30 * time.Second),
	}
	if AllowComment(now, recent) {
		t.Fatal("expected comment to be rejected")
	}
}
