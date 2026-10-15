package articles

import (
	"context"
	"log/slog"
	"time"
)

// DefaultArticleLogRetention 是每篇文章默认保留的修订快照数量（不含首版）。
const DefaultArticleLogRetention = 20

const dailyLogPruneDelay = 15 * time.Minute

// ArticleLogPruner 由 sqlc Queries 实现；保留策略只依赖这一个方法。
type ArticleLogPruner interface {
	PruneArticleLogs(ctx context.Context, keepRevisions int32) (int64, error)
}

// PruneStaleArticleLogs 按保留策略删除过期的文章修订快照，返回删除行数。
// keep 小于 1 时回退到默认保留数量，避免配置错误导致修订历史被清空。
func PruneStaleArticleLogs(ctx context.Context, pruner ArticleLogPruner, keep int32) (int64, error) {
	if keep < 1 {
		keep = DefaultArticleLogRetention
	}
	return pruner.PruneArticleLogs(ctx, keep)
}

func StartDailyArticleLogPruneScheduler(ctx context.Context, pruner ArticleLogPruner, keep int32, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	go runDailyArticleLogPruneScheduler(ctx, pruner, keep, logger, time.Now)
}

func runDailyArticleLogPruneScheduler(ctx context.Context, pruner ArticleLogPruner, keep int32, logger *slog.Logger, now func() time.Time) {
	prune := func() {
		start := now()
		logger.Info("starting article log prune", "time", start)
		deleted, err := PruneStaleArticleLogs(ctx, pruner, keep)
		if err != nil {
			logger.Error("prune article logs failed", "error", err, "duration", now().Sub(start))
			return
		}
		logger.Info("pruned stale article logs", "deleted", deleted, "duration", now().Sub(start), "next", nextDailyArticleLogPruneTime(now()))
	}

	prune()
	for {
		next := nextDailyArticleLogPruneTime(now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			prune()
		}
	}
}

func nextDailyArticleLogPruneTime(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(dailyLogPruneDelay)
	if now.Before(today) {
		return today
	}
	return today.AddDate(0, 0, 1)
}
