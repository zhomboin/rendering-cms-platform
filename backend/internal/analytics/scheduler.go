package analytics

import (
	"context"
	"log/slog"
	"time"
)

const dailyViewArchiveDelay = 5 * time.Minute

func StartDailyViewArchiveScheduler(ctx context.Context, archiver DailyViewArchiver, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	go runDailyViewArchiveScheduler(ctx, archiver, logger, time.Now)
}

func runDailyViewArchiveScheduler(ctx context.Context, archiver DailyViewArchiver, logger *slog.Logger, now func() time.Time) {
	archive := func() {
		start := now()
		logger.Info("starting daily view archive", "time", start)
		if err := ArchivePastDailyViews(ctx, archiver, start); err != nil {
			logger.Error("archive daily views failed", "error", err, "duration", now().Sub(start))
			return
		}
		logger.Info("archived stale daily views", "duration", now().Sub(start), "next", nextDailyViewArchiveTime(now()))
	}

	archive()
	for {
		next := nextDailyViewArchiveTime(now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			archive()
		}
	}
}

func nextDailyViewArchiveTime(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(dailyViewArchiveDelay)
	if now.Before(today) {
		return today
	}
	return today.AddDate(0, 0, 1)
}
