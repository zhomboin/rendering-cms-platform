package auth

import "time"

type LoginLockoutDecision struct {
	Locked      bool
	Duration    time.Duration
	LockedUntil time.Time
}

type loginLockoutRule struct {
	window   time.Duration
	limit    int
	duration time.Duration
}

var loginLockoutRules = []loginLockoutRule{
	{window: 5 * time.Minute, limit: 5, duration: 5 * time.Minute},
	{window: 15 * time.Minute, limit: 10, duration: 15 * time.Minute},
	{window: time.Hour, limit: 20, duration: 24 * time.Hour},
}

func EvaluateLoginLockout(now time.Time, recentFailures []time.Time) LoginLockoutDecision {
	var decision LoginLockoutDecision
	for _, rule := range loginLockoutRules {
		count := 0
		var latest time.Time
		for _, failedAt := range recentFailures {
			if now.Sub(failedAt) <= rule.window {
				count++
				if failedAt.After(latest) {
					latest = failedAt
				}
			}
		}
		if count >= rule.limit {
			lockedUntil := latest.Add(rule.duration)
			if now.Before(lockedUntil) && (!decision.Locked || lockedUntil.After(decision.LockedUntil)) {
				decision = LoginLockoutDecision{
					Locked:      true,
					Duration:    rule.duration,
					LockedUntil: lockedUntil,
				}
			}
		}
	}
	return decision
}
