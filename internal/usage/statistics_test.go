package usage

import (
	"context"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/redisqueue"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestTokenUsageAggregatorGroupsByProviderAndPeriods(t *testing.T) {
	prevUsageEnabled := redisqueue.UsageStatisticsEnabled()
	redisqueue.SetUsageStatisticsEnabled(true)
	t.Cleanup(func() { redisqueue.SetUsageStatisticsEnabled(prevUsageEnabled) })

	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local)
	aggregator := newTokenUsageAggregator(func() time.Time { return now })

	aggregator.HandleUsage(context.Background(), coreusage.Record{
		Provider:    " OpenAI ",
		RequestedAt: now.Add(-1 * time.Hour),
		Detail: coreusage.Detail{
			InputTokens:     10,
			OutputTokens:    20,
			ReasoningTokens: 5,
		},
	})
	aggregator.HandleUsage(context.Background(), coreusage.Record{
		Provider:    "codex",
		RequestedAt: now.AddDate(0, -1, 0),
		Failed:      true,
		Detail: coreusage.Detail{
			InputTokens:  7,
			OutputTokens: 8,
			TotalTokens:  99,
		},
	})

	snapshot := aggregator.Snapshot()
	if snapshot.Total.Requests != 2 {
		t.Fatalf("total requests = %d, want 2", snapshot.Total.Requests)
	}
	if snapshot.Total.FailedRequests != 1 {
		t.Fatalf("failed requests = %d, want 1", snapshot.Total.FailedRequests)
	}
	if snapshot.Total.Tokens.TotalTokens != 134 {
		t.Fatalf("total tokens = %d, want 134", snapshot.Total.Tokens.TotalTokens)
	}
	if snapshot.Day.Requests != 1 {
		t.Fatalf("day requests = %d, want 1", snapshot.Day.Requests)
	}
	if snapshot.Week.Requests != 1 {
		t.Fatalf("week requests = %d, want 1", snapshot.Week.Requests)
	}
	if snapshot.Month.Requests != 1 {
		t.Fatalf("month requests = %d, want 1", snapshot.Month.Requests)
	}

	openai := requireProvider(t, snapshot, "openai")
	if openai.Day.Tokens.TotalTokens != 35 {
		t.Fatalf("openai day tokens = %d, want 35", openai.Day.Tokens.TotalTokens)
	}
	if openai.Total.SuccessfulRequests != 1 {
		t.Fatalf("openai successful requests = %d, want 1", openai.Total.SuccessfulRequests)
	}

	codex := requireProvider(t, snapshot, "codex")
	if codex.Total.FailedRequests != 1 {
		t.Fatalf("codex failed requests = %d, want 1", codex.Total.FailedRequests)
	}
	if codex.Day.Requests != 0 {
		t.Fatalf("codex day requests = %d, want 0", codex.Day.Requests)
	}
}

func TestTokenUsageAggregatorHonorsUsageStatisticsToggle(t *testing.T) {
	prevUsageEnabled := redisqueue.UsageStatisticsEnabled()
	redisqueue.SetUsageStatisticsEnabled(false)
	t.Cleanup(func() { redisqueue.SetUsageStatisticsEnabled(prevUsageEnabled) })

	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.Local)
	aggregator := newTokenUsageAggregator(func() time.Time { return now })

	aggregator.HandleUsage(context.Background(), coreusage.Record{
		Provider:    "openai",
		RequestedAt: now,
		Detail: coreusage.Detail{
			TotalTokens: 10,
		},
	})

	snapshot := aggregator.Snapshot()
	if snapshot.Total.Requests != 0 {
		t.Fatalf("total requests = %d, want 0", snapshot.Total.Requests)
	}
	if snapshot.Enabled {
		t.Fatal("snapshot enabled = true, want false")
	}
}

func requireProvider(t *testing.T, snapshot StatisticsSnapshot, provider string) ProviderUsageStatistics {
	t.Helper()
	for _, item := range snapshot.Providers {
		if item.Provider == provider {
			return item
		}
	}
	t.Fatalf("provider %q not found in %#v", provider, snapshot.Providers)
	return ProviderUsageStatistics{}
}
