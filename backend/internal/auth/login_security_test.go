package auth

import (
	"testing"
	"time"
)

func TestLoginLockoutUsesFiveMinuteRule(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	failures := []time.Time{
		now.Add(-4 * time.Minute),
		now.Add(-3 * time.Minute),
		now.Add(-2 * time.Minute),
		now.Add(-1 * time.Minute),
		now.Add(-10 * time.Second),
	}

	decision := EvaluateLoginLockout(now, failures)

	if !decision.Locked {
		t.Fatal("expected login to be locked")
	}
	if decision.Duration != 5*time.Minute {
		t.Fatalf("duration = %s, want 5m", decision.Duration)
	}
}

func TestLoginLockoutUsesFifteenMinuteRule(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	failures := make([]time.Time, 0, 10)
	for i := 0; i < 10; i++ {
		failures = append(failures, now.Add(-time.Duration(i)*time.Minute))
	}

	decision := EvaluateLoginLockout(now, failures)

	if !decision.Locked {
		t.Fatal("expected login to be locked")
	}
	if decision.Duration != 15*time.Minute {
		t.Fatalf("duration = %s, want 15m", decision.Duration)
	}
}

func TestLoginLockoutUsesOneDayRule(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	failures := make([]time.Time, 0, 20)
	for i := 0; i < 20; i++ {
		failures = append(failures, now.Add(-time.Duration(i)*time.Minute))
	}

	decision := EvaluateLoginLockout(now, failures)

	if !decision.Locked {
		t.Fatal("expected login to be locked")
	}
	if decision.Duration != 24*time.Hour {
		t.Fatalf("duration = %s, want 24h", decision.Duration)
	}
}

func TestLoginLockoutAllowsBelowThreshold(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	failures := []time.Time{
		now.Add(-4 * time.Minute),
		now.Add(-3 * time.Minute),
		now.Add(-2 * time.Minute),
		now.Add(-1 * time.Minute),
	}

	decision := EvaluateLoginLockout(now, failures)

	if decision.Locked {
		t.Fatalf("expected login to be allowed, got lock until %s", decision.LockedUntil)
	}
}
