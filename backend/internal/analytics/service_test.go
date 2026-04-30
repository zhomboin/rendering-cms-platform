package analytics

import "testing"

func TestSummaryTotals(t *testing.T) {
	days := []DailyView{
		{Views: 10},
		{Views: 20},
		{Views: 7},
	}

	if total := TotalViews(days); total != 37 {
		t.Fatalf("TotalViews() = %d, want 37", total)
	}
}
