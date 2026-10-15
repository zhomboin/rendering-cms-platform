package articles

import (
	"context"
	"errors"
	"testing"
	"time"
)

type articleLogPrunerStub struct {
	keepArg  int32
	keepSeen bool
	deleted  int64
	err      error
}

func (s *articleLogPrunerStub) PruneArticleLogs(ctx context.Context, keepRevisions int32) (int64, error) {
	s.keepSeen = true
	s.keepArg = keepRevisions
	return s.deleted, s.err
}

func TestPruneStaleArticleLogsPassesKeepLimit(t *testing.T) {
	pruner := &articleLogPrunerStub{deleted: 7}
	deleted, err := PruneStaleArticleLogs(context.Background(), pruner, 20)
	if err != nil {
		t.Fatalf("PruneStaleArticleLogs error = %v", err)
	}
	if deleted != 7 {
		t.Fatalf("deleted = %d, want 7", deleted)
	}
	if !pruner.keepSeen || pruner.keepArg != 20 {
		t.Fatalf("keep arg = %d (seen %v), want 20", pruner.keepArg, pruner.keepSeen)
	}
}

func TestPruneStaleArticleLogsFallsBackToDefaultOnInvalidKeep(t *testing.T) {
	pruner := &articleLogPrunerStub{}
	if _, err := PruneStaleArticleLogs(context.Background(), pruner, 0); err != nil {
		t.Fatalf("PruneStaleArticleLogs error = %v", err)
	}
	if pruner.keepArg != DefaultArticleLogRetention {
		t.Fatalf("keep arg = %d, want default %d", pruner.keepArg, DefaultArticleLogRetention)
	}
}

func TestPruneStaleArticleLogsReturnsStoreError(t *testing.T) {
	pruner := &articleLogPrunerStub{err: errors.New("database unavailable")}
	if _, err := PruneStaleArticleLogs(context.Background(), pruner, 20); err == nil {
		t.Fatal("PruneStaleArticleLogs error = nil, want store error")
	}
}

func TestNextDailyArticleLogPruneTimeUsesNextMidnightWindow(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 10, 15, 23, 0, 0, 0, location)
	want := time.Date(2026, 10, 16, 0, 15, 0, 0, location)
	if got := nextDailyArticleLogPruneTime(now); !got.Equal(want) {
		t.Fatalf("nextDailyArticleLogPruneTime() = %s, want %s", got, want)
	}

	morning := time.Date(2026, 10, 15, 6, 0, 0, 0, location)
	wantToday := time.Date(2026, 10, 15, 0, 15, 0, 0, location)
	if got := nextDailyArticleLogPruneTime(morning); !got.Equal(wantToday.AddDate(0, 0, 1)) {
		t.Fatalf("nextDailyArticleLogPruneTime(morning) = %s, want next day %s", got, wantToday.AddDate(0, 0, 1))
	}
}
