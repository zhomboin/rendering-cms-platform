package analytics

import "time"

type DailyView struct {
	Date  time.Time
	Views int
}

func TotalViews(days []DailyView) int {
	total := 0
	for _, day := range days {
		total += day.Views
	}
	return total
}
