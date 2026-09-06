package usage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/redisqueue"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

const (
	maxDayBuckets   = 90
	maxWeekBuckets  = 104
	maxMonthBuckets = 36
)

var defaultTokenUsageStats = newTokenUsageAggregator(time.Now)

func init() {
	coreusage.RegisterNamedPlugin("token_usage_statistics", defaultTokenUsageStats)
}

// TokenCounts contains the token breakdown accumulated from usage records.
type TokenCounts struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	ReasoningTokens     int64 `json:"reasoning_tokens"`
	CachedTokens        int64 `json:"cached_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
}

// UsageBucket contains request and token totals for one aggregation bucket.
type UsageBucket struct {
	Requests           int64       `json:"requests"`
	SuccessfulRequests int64       `json:"successful_requests"`
	FailedRequests     int64       `json:"failed_requests"`
	Tokens             TokenCounts `json:"tokens"`
}

// PeriodUsageBucket contains request and token totals for a named time window.
type PeriodUsageBucket struct {
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	UsageBucket
}

// ProviderUsageStatistics contains total and period usage for one AI provider.
type ProviderUsageStatistics struct {
	Provider string            `json:"provider"`
	Total    UsageBucket       `json:"total"`
	Day      PeriodUsageBucket `json:"day"`
	Week     PeriodUsageBucket `json:"week"`
	Month    PeriodUsageBucket `json:"month"`
}

// StatisticsSnapshot is the management API response for token usage statistics.
type StatisticsSnapshot struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Enabled     bool                       `json:"enabled"`
	Total       UsageBucket                `json:"total"`
	Day         PeriodUsageBucket          `json:"day"`
	Week        PeriodUsageBucket          `json:"week"`
	Month       PeriodUsageBucket          `json:"month"`
	Providers   []ProviderUsageStatistics  `json:"providers"`
	Periods     map[string]PeriodRangeInfo `json:"periods"`
}

// PeriodRangeInfo describes the wall-clock range used by one period bucket.
type PeriodRangeInfo struct {
	Label string    `json:"label"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type tokenUsageAggregator struct {
	now       func() time.Time
	mu        sync.RWMutex
	providers map[string]*providerCounters
}

type providerCounters struct {
	total  usageCounter
	days   map[string]usageCounter
	weeks  map[string]usageCounter
	months map[string]usageCounter
}

type usageCounter struct {
	requests int64
	failed   int64
	tokens   TokenCounts
}

type periodRange struct {
	key   string
	label string
	start time.Time
	end   time.Time
}

func newTokenUsageAggregator(now func() time.Time) *tokenUsageAggregator {
	if now == nil {
		now = time.Now
	}
	return &tokenUsageAggregator{
		now:       now,
		providers: make(map[string]*providerCounters),
	}
}

// Snapshot returns persisted token usage statistics when available, with an in-memory fallback.
func Snapshot() StatisticsSnapshot {
	if snapshot, ok := snapshotFromSQLite(); ok {
		return snapshot
	}
	return defaultTokenUsageStats.Snapshot()
}

func (a *tokenUsageAggregator) HandleUsage(ctx context.Context, record coreusage.Record) {
	_ = ctx
	if a == nil || !redisqueue.UsageStatisticsEnabled() {
		return
	}

	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = a.now()
	}

	provider := normalizeProvider(record.Provider)
	counter := usageCounter{
		requests: 1,
		tokens:   tokensFromDetail(record.Detail),
	}
	if record.Failed || record.Fail.StatusCode >= 400 {
		counter.failed = 1
	}

	day := dayRange(timestamp)
	week := weekRange(timestamp)
	month := monthRange(timestamp)

	event := usageEvent{
		provider: provider,
		total:    counter,
		dayKey:   day.key,
		weekKey:  week.key,
		monthKey: month.key,
	}
	if store := currentSQLiteStore(); store != nil {
		store.enqueue(event)
	}

	a.mu.Lock()
	state := a.providers[provider]
	if state == nil {
		state = &providerCounters{
			days:   make(map[string]usageCounter),
			weeks:  make(map[string]usageCounter),
			months: make(map[string]usageCounter),
		}
		a.providers[provider] = state
	}

	state.total.add(counter)
	addToCounterMap(state.days, day.key, counter)
	addToCounterMap(state.weeks, week.key, counter)
	addToCounterMap(state.months, month.key, counter)
	pruneCounterMap(state.days, maxDayBuckets)
	pruneCounterMap(state.weeks, maxWeekBuckets)
	pruneCounterMap(state.months, maxMonthBuckets)
	a.mu.Unlock()
}

func (a *tokenUsageAggregator) Snapshot() StatisticsSnapshot {
	if a == nil {
		return StatisticsSnapshot{}
	}

	now := a.now()
	day := dayRange(now)
	week := weekRange(now)
	month := monthRange(now)

	snapshot := StatisticsSnapshot{
		GeneratedAt: now,
		Enabled:     redisqueue.UsageStatisticsEnabled(),
		Day:         emptyPeriodUsageBucket(day),
		Week:        emptyPeriodUsageBucket(week),
		Month:       emptyPeriodUsageBucket(month),
		Periods: map[string]PeriodRangeInfo{
			"day":   rangeInfo(day),
			"week":  rangeInfo(week),
			"month": rangeInfo(month),
		},
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	providers := make([]string, 0, len(a.providers))
	for provider := range a.providers {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	snapshot.Providers = make([]ProviderUsageStatistics, 0, len(providers))
	for _, provider := range providers {
		state := a.providers[provider]
		if state == nil {
			continue
		}
		providerStats := ProviderUsageStatistics{
			Provider: provider,
			Total:    state.total.toBucket(),
			Day:      periodUsageBucket(day, state.days[day.key]),
			Week:     periodUsageBucket(week, state.weeks[week.key]),
			Month:    periodUsageBucket(month, state.months[month.key]),
		}
		snapshot.Total.add(providerStats.Total)
		snapshot.Day.UsageBucket.add(providerStats.Day.UsageBucket)
		snapshot.Week.UsageBucket.add(providerStats.Week.UsageBucket)
		snapshot.Month.UsageBucket.add(providerStats.Month.UsageBucket)
		snapshot.Providers = append(snapshot.Providers, providerStats)
	}

	sort.SliceStable(snapshot.Providers, func(i, j int) bool {
		left := snapshot.Providers[i]
		right := snapshot.Providers[j]
		if left.Total.Tokens.TotalTokens == right.Total.Tokens.TotalTokens {
			return left.Provider < right.Provider
		}
		return left.Total.Tokens.TotalTokens > right.Total.Tokens.TotalTokens
	})

	return snapshot
}

func normalizeProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return "unknown"
	}
	return provider
}

func tokensFromDetail(detail coreusage.Detail) TokenCounts {
	tokens := TokenCounts{
		InputTokens:         detail.InputTokens,
		OutputTokens:        detail.OutputTokens,
		ReasoningTokens:     detail.ReasoningTokens,
		CachedTokens:        detail.CachedTokens,
		CacheReadTokens:     detail.CacheReadTokens,
		CacheCreationTokens: detail.CacheCreationTokens,
		TotalTokens:         detail.TotalTokens,
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens + tokens.ReasoningTokens
	}
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens + tokens.ReasoningTokens + tokens.CachedTokens
	}
	return tokens
}

func addToCounterMap(counters map[string]usageCounter, key string, counter usageCounter) {
	current := counters[key]
	current.add(counter)
	counters[key] = current
}

func pruneCounterMap(counters map[string]usageCounter, maxBuckets int) {
	if len(counters) <= maxBuckets || maxBuckets <= 0 {
		return
	}
	keys := make([]string, 0, len(counters))
	for key := range counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys[:len(keys)-maxBuckets] {
		delete(counters, key)
	}
}

func (c *usageCounter) add(other usageCounter) {
	c.requests += other.requests
	c.failed += other.failed
	c.tokens.add(other.tokens)
}

func (c usageCounter) toBucket() UsageBucket {
	failed := c.failed
	if failed < 0 {
		failed = 0
	}
	if failed > c.requests {
		failed = c.requests
	}
	return UsageBucket{
		Requests:           c.requests,
		SuccessfulRequests: c.requests - failed,
		FailedRequests:     failed,
		Tokens:             c.tokens,
	}
}

func (b *UsageBucket) add(other UsageBucket) {
	b.Requests += other.Requests
	b.SuccessfulRequests += other.SuccessfulRequests
	b.FailedRequests += other.FailedRequests
	b.Tokens.add(other.Tokens)
}

func (t *TokenCounts) add(other TokenCounts) {
	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.ReasoningTokens += other.ReasoningTokens
	t.CachedTokens += other.CachedTokens
	t.CacheReadTokens += other.CacheReadTokens
	t.CacheCreationTokens += other.CacheCreationTokens
	t.TotalTokens += other.TotalTokens
}

func emptyPeriodUsageBucket(period periodRange) PeriodUsageBucket {
	return periodUsageBucket(period, usageCounter{})
}

func periodUsageBucket(period periodRange, counter usageCounter) PeriodUsageBucket {
	return PeriodUsageBucket{
		Label:       period.label,
		Start:       period.start,
		End:         period.end,
		UsageBucket: counter.toBucket(),
	}
}

func rangeInfo(period periodRange) PeriodRangeInfo {
	return PeriodRangeInfo{
		Label: period.label,
		Start: period.start,
		End:   period.end,
	}
}

func dayRange(t time.Time) periodRange {
	local := t.In(time.Local)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	return periodRange{
		key:   start.Format("2006-01-02"),
		label: start.Format("2006-01-02"),
		start: start,
		end:   start.AddDate(0, 0, 1),
	}
}

func weekRange(t time.Time) periodRange {
	local := t.In(time.Local)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	daysFromMonday := (int(dayStart.Weekday()) + 6) % 7
	start := dayStart.AddDate(0, 0, -daysFromMonday)
	isoYear, isoWeek := local.ISOWeek()
	label := fmt.Sprintf("%04d-W%02d", isoYear, isoWeek)
	return periodRange{
		key:   label,
		label: label,
		start: start,
		end:   start.AddDate(0, 0, 7),
	}
}

func monthRange(t time.Time) periodRange {
	local := t.In(time.Local)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, local.Location())
	return periodRange{
		key:   start.Format("2006-01"),
		label: start.Format("2006-01"),
		start: start,
		end:   start.AddDate(0, 1, 0),
	}
}
