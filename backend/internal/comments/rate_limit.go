package comments

import "time"

func AllowComment(now time.Time, recent []time.Time) bool {
	count := 0
	for _, createdAt := range recent {
		if now.Sub(createdAt) <= time.Minute {
			count++
		}
	}
	return count < 3
}
