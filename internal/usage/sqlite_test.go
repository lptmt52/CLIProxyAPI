package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteUsageStorePersistsAggregates(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 30, 0, 0, time.Local)
	path := filepath.Join(t.TempDir(), "usage.db")
	store, errOpen := openSQLiteUsageStore(path)
	if errOpen != nil {
		t.Fatalf("openSQLiteUsageStore() error = %v", errOpen)
	}

	day := dayRange(now)
	week := weekRange(now)
	month := monthRange(now)
	store.enqueue(usageEvent{
		provider: "openai",
		total: usageCounter{
			requests: 1,
			tokens: TokenCounts{
				InputTokens:  10,
				OutputTokens: 20,
				TotalTokens:  30,
			},
		},
		dayKey:   day.key,
		weekKey:  week.key,
		monthKey: month.key,
	})
	store.enqueue(usageEvent{
		provider: "openai",
		total: usageCounter{
			requests: 1,
			failed:   1,
			tokens: TokenCounts{
				InputTokens:     5,
				OutputTokens:    7,
				ReasoningTokens: 3,
				TotalTokens:     15,
			},
		},
		dayKey:   day.key,
		weekKey:  week.key,
		monthKey: month.key,
	})
	if errFlush := store.flush(); errFlush != nil {
		store.close()
		t.Fatalf("flush() error = %v", errFlush)
	}

	snapshot, errSnapshot := store.snapshot(now)
	if errSnapshot != nil {
		store.close()
		t.Fatalf("snapshot() error = %v", errSnapshot)
	}
	provider := requireProvider(t, snapshot, "openai")
	if provider.Total.Requests != 2 || provider.Total.FailedRequests != 1 {
		store.close()
		t.Fatalf("total request counters = %#v, want requests=2 failed=1", provider.Total)
	}
	if provider.Total.Tokens.TotalTokens != 45 || provider.Day.Tokens.TotalTokens != 45 {
		store.close()
		t.Fatalf("token totals = total %d day %d, want 45", provider.Total.Tokens.TotalTokens, provider.Day.Tokens.TotalTokens)
	}
	store.close()

	if _, errStat := os.Stat(path); errStat != nil {
		t.Fatalf("usage database file missing: %v", errStat)
	}
	reopened, errReopen := openSQLiteUsageStore(path)
	if errReopen != nil {
		t.Fatalf("reopen usage database: %v", errReopen)
	}
	t.Cleanup(reopened.close)
	persisted, errPersisted := reopened.snapshot(now)
	if errPersisted != nil {
		t.Fatalf("persisted snapshot() error = %v", errPersisted)
	}
	persistedProvider := requireProvider(t, persisted, "openai")
	if persistedProvider.Total.Tokens.TotalTokens != 45 || persistedProvider.Month.Requests != 2 {
		t.Fatalf("persisted provider = %#v, want total_tokens=45 month_requests=2", persistedProvider)
	}
}
