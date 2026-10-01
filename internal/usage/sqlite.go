package usage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/redisqueue"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	usageSQLiteSchema = `
CREATE TABLE IF NOT EXISTS usage_totals (
    provider TEXT PRIMARY KEY,
    requests INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS usage_buckets (
    period TEXT NOT NULL,
    bucket_key TEXT NOT NULL,
    provider TEXT NOT NULL,
    requests INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (period, bucket_key, provider)
);
CREATE INDEX IF NOT EXISTS idx_usage_buckets_period_key
    ON usage_buckets (period, bucket_key);
`
	usageSQLiteUpsertTotal = `
INSERT INTO usage_totals (
    provider, requests, failed, input_tokens, output_tokens, reasoning_tokens,
    cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(provider) DO UPDATE SET
    requests = requests + excluded.requests,
    failed = failed + excluded.failed,
    input_tokens = input_tokens + excluded.input_tokens,
    output_tokens = output_tokens + excluded.output_tokens,
    reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
    cached_tokens = cached_tokens + excluded.cached_tokens,
    cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    total_tokens = total_tokens + excluded.total_tokens
`
	usageSQLiteUpsertBucket = `
INSERT INTO usage_buckets (
    period, bucket_key, provider, requests, failed, input_tokens, output_tokens,
    reasoning_tokens, cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(period, bucket_key, provider) DO UPDATE SET
    requests = requests + excluded.requests,
    failed = failed + excluded.failed,
    input_tokens = input_tokens + excluded.input_tokens,
    output_tokens = output_tokens + excluded.output_tokens,
    reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
    cached_tokens = cached_tokens + excluded.cached_tokens,
    cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    total_tokens = total_tokens + excluded.total_tokens
`
	usageSQLiteCounterSelect = `
SELECT provider, requests, failed, input_tokens, output_tokens, reasoning_tokens,
       cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
FROM usage_totals
ORDER BY provider
`
	usageSQLiteBucketSelect = `
SELECT provider, requests, failed, input_tokens, output_tokens, reasoning_tokens,
       cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens
FROM usage_buckets
WHERE period = ? AND bucket_key = ?
`
)

var (
	sqliteStoreMu   sync.RWMutex
	sqliteStore     *sqliteUsageStore
	sqliteStorePath string
)

type usageEvent struct {
	provider string
	total    usageCounter
	dayKey   string
	weekKey  string
	monthKey string
}

type sqliteUsageStore struct {
	db              *sql.DB
	queue           chan usageEvent
	flushRequests   chan chan error
	stop            chan struct{}
	done            chan struct{}
	closeOnce       sync.Once
	lastCleanupTime time.Time
}

// ConfigureSQLite opens the local token usage database and starts its async writer.
func ConfigureSQLite(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	absolutePath, errAbs := filepath.Abs(path)
	if errAbs != nil {
		return fmt.Errorf("resolve usage database path: %w", errAbs)
	}

	sqliteStoreMu.RLock()
	alreadyConfigured := sqliteStore != nil && sqliteStorePath == absolutePath
	sqliteStoreMu.RUnlock()
	if alreadyConfigured {
		return nil
	}

	store, errOpen := openSQLiteUsageStore(absolutePath)
	if errOpen != nil {
		return errOpen
	}

	sqliteStoreMu.Lock()
	oldStore := sqliteStore
	sqliteStore = store
	sqliteStorePath = absolutePath
	sqliteStoreMu.Unlock()
	if oldStore != nil {
		go oldStore.close()
	}
	return nil
}

func currentSQLiteStore() *sqliteUsageStore {
	sqliteStoreMu.RLock()
	defer sqliteStoreMu.RUnlock()
	return sqliteStore
}

// CloseSQLite flushes and closes the configured token usage database.
func CloseSQLite() {
	sqliteStoreMu.Lock()
	store := sqliteStore
	sqliteStore = nil
	sqliteStorePath = ""
	sqliteStoreMu.Unlock()
	if store != nil {
		store.close()
	}
}

func openSQLiteUsageStore(path string) (*sqliteUsageStore, error) {
	if errMkdir := os.MkdirAll(filepath.Dir(path), 0o755); errMkdir != nil {
		return nil, fmt.Errorf("create usage database directory: %w", errMkdir)
	}

	db, errOpen := sql.Open("sqlite", path)
	if errOpen != nil {
		return nil, fmt.Errorf("open usage database: %w", errOpen)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	closeDB := func(err error) (*sqliteUsageStore, error) {
		_ = db.Close()
		return nil, err
	}

	if _, errExec := db.Exec("PRAGMA busy_timeout = 5000"); errExec != nil {
		return closeDB(fmt.Errorf("configure usage database timeout: %w", errExec))
	}
	if _, errExec := db.Exec("PRAGMA journal_mode = WAL"); errExec != nil {
		return closeDB(fmt.Errorf("configure usage database journal: %w", errExec))
	}
	if _, errExec := db.Exec("PRAGMA synchronous = NORMAL"); errExec != nil {
		return closeDB(fmt.Errorf("configure usage database sync: %w", errExec))
	}
	if _, errExec := db.Exec(usageSQLiteSchema); errExec != nil {
		return closeDB(fmt.Errorf("initialize usage database schema: %w", errExec))
	}

	store := &sqliteUsageStore{
		db:            db,
		queue:         make(chan usageEvent, 4096),
		flushRequests: make(chan chan error),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go store.run()
	return store, nil
}

func (s *sqliteUsageStore) enqueue(event usageEvent) {
	if s == nil {
		return
	}
	select {
	case s.queue <- event:
	case <-s.stop:
	}
}

func (s *sqliteUsageStore) run() {
	defer close(s.done)
	for {
		select {
		case event := <-s.queue:
			if errWrite := s.writeBatch(s.collectBatch(event)); errWrite != nil {
				log.WithError(errWrite).Warn("failed to persist token usage batch")
			}
		case request := <-s.flushRequests:
			request <- s.flushQueue()
		case <-s.stop:
			if errFlush := s.flushQueue(); errFlush != nil {
				log.WithError(errFlush).Warn("failed to flush token usage database during shutdown")
			}
			return
		}
	}
}

func (s *sqliteUsageStore) collectBatch(first usageEvent) []usageEvent {
	batch := make([]usageEvent, 0, 128)
	batch = append(batch, first)
	timer := time.NewTimer(25 * time.Millisecond)
	defer timer.Stop()
	for len(batch) < 128 {
		select {
		case event := <-s.queue:
			batch = append(batch, event)
		case <-timer.C:
			return batch
		}
	}
	return batch
}

func (s *sqliteUsageStore) flush() error {
	if s == nil {
		return nil
	}
	request := make(chan error, 1)
	select {
	case s.flushRequests <- request:
		return <-request
	case <-s.stop:
		return errors.New("usage database is closed")
	}
}

func (s *sqliteUsageStore) flushQueue() error {
	var firstErr error
	for {
		batch := make([]usageEvent, 0, 128)
		for len(batch) < cap(batch) {
			select {
			case event := <-s.queue:
				batch = append(batch, event)
			default:
				goto batchDrained
			}
		}
	batchDrained:
		if len(batch) == 0 {
			return firstErr
		}
		if errWrite := s.writeBatch(batch); errWrite != nil && firstErr == nil {
			firstErr = errWrite
		}
	}
}

func (s *sqliteUsageStore) writeBatch(batch []usageEvent) error {
	if len(batch) == 0 {
		return nil
	}
	tx, errBegin := s.db.Begin()
	if errBegin != nil {
		return fmt.Errorf("begin usage database transaction: %w", errBegin)
	}
	rollback := func(err error) error {
		_ = tx.Rollback()
		return err
	}
	for _, event := range batch {
		if _, errExec := tx.Exec(usageSQLiteUpsertTotal, append([]any{event.provider}, counterValues(event.total)...)...); errExec != nil {
			return rollback(fmt.Errorf("write usage total: %w", errExec))
		}
		for _, bucket := range []struct {
			period string
			key    string
		}{
			{period: "day", key: event.dayKey},
			{period: "week", key: event.weekKey},
			{period: "month", key: event.monthKey},
		} {
			args := append([]any{bucket.period, bucket.key, event.provider}, counterValues(event.total)...)
			if _, errExec := tx.Exec(usageSQLiteUpsertBucket, args...); errExec != nil {
				return rollback(fmt.Errorf("write usage bucket: %w", errExec))
			}
		}
	}
	if s.lastCleanupTime.IsZero() || time.Since(s.lastCleanupTime) >= time.Hour {
		if errCleanup := cleanupUsageBuckets(tx, time.Now()); errCleanup != nil {
			return rollback(errCleanup)
		}
		s.lastCleanupTime = time.Now()
	}
	if errCommit := tx.Commit(); errCommit != nil {
		return fmt.Errorf("commit usage database transaction: %w", errCommit)
	}
	return nil
}

func cleanupUsageBuckets(tx *sql.Tx, now time.Time) error {
	dayCutoff := dayRange(now.AddDate(0, 0, -maxDayBuckets)).key
	weekCutoff := weekRange(now.AddDate(0, 0, -7*maxWeekBuckets)).key
	monthCutoff := monthRange(now.AddDate(0, -maxMonthBuckets, 0)).key
	if _, err := tx.Exec("DELETE FROM usage_buckets WHERE period = 'day' AND bucket_key < ?", dayCutoff); err != nil {
		return fmt.Errorf("prune daily usage buckets: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM usage_buckets WHERE period = 'week' AND bucket_key < ?", weekCutoff); err != nil {
		return fmt.Errorf("prune weekly usage buckets: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM usage_buckets WHERE period = 'month' AND bucket_key < ?", monthCutoff); err != nil {
		return fmt.Errorf("prune monthly usage buckets: %w", err)
	}
	return nil
}

func counterValues(counter usageCounter) []any {
	return []any{
		counter.requests,
		counter.failed,
		counter.tokens.InputTokens,
		counter.tokens.OutputTokens,
		counter.tokens.ReasoningTokens,
		counter.tokens.CachedTokens,
		counter.tokens.CacheReadTokens,
		counter.tokens.CacheCreationTokens,
		counter.tokens.TotalTokens,
	}
}

func (s *sqliteUsageStore) snapshot(now time.Time) (StatisticsSnapshot, error) {
	day := dayRange(now)
	week := weekRange(now)
	month := monthRange(now)
	states := make(map[string]*providerCounters)
	if err := s.loadCounters(states, func(state *providerCounters, counter usageCounter) {
		state.total = counter
	}, usageSQLiteCounterSelect); err != nil {
		return StatisticsSnapshot{}, err
	}
	if err := s.loadCounters(states, func(state *providerCounters, counter usageCounter) {
		state.days[day.key] = counter
	}, usageSQLiteBucketSelect, "day", day.key); err != nil {
		return StatisticsSnapshot{}, err
	}
	if err := s.loadCounters(states, func(state *providerCounters, counter usageCounter) {
		state.weeks[week.key] = counter
	}, usageSQLiteBucketSelect, "week", week.key); err != nil {
		return StatisticsSnapshot{}, err
	}
	if err := s.loadCounters(states, func(state *providerCounters, counter usageCounter) {
		state.months[month.key] = counter
	}, usageSQLiteBucketSelect, "month", month.key); err != nil {
		return StatisticsSnapshot{}, err
	}

	providers := make([]string, 0, len(states))
	for provider := range states {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
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
		Providers: make([]ProviderUsageStatistics, 0, len(providers)),
	}
	for _, provider := range providers {
		state := states[provider]
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
	return snapshot, nil
}

func (s *sqliteUsageStore) loadCounters(states map[string]*providerCounters, assign func(*providerCounters, usageCounter), query string, args ...any) error {
	rows, errQuery := s.db.Query(query, args...)
	if errQuery != nil {
		return fmt.Errorf("query usage database: %w", errQuery)
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var counter usageCounter
		if errScan := rows.Scan(
			&provider,
			&counter.requests,
			&counter.failed,
			&counter.tokens.InputTokens,
			&counter.tokens.OutputTokens,
			&counter.tokens.ReasoningTokens,
			&counter.tokens.CachedTokens,
			&counter.tokens.CacheReadTokens,
			&counter.tokens.CacheCreationTokens,
			&counter.tokens.TotalTokens,
		); errScan != nil {
			return fmt.Errorf("scan usage database row: %w", errScan)
		}
		state := states[provider]
		if state == nil {
			state = &providerCounters{
				days:   make(map[string]usageCounter),
				weeks:  make(map[string]usageCounter),
				months: make(map[string]usageCounter),
			}
			states[provider] = state
		}
		assign(state, counter)
	}
	if errRows := rows.Err(); errRows != nil {
		return fmt.Errorf("iterate usage database rows: %w", errRows)
	}
	return nil
}

func (s *sqliteUsageStore) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.stop)
		<-s.done
		if errClose := s.db.Close(); errClose != nil {
			log.WithError(errClose).Warn("failed to close token usage SQLite database")
		}
	})
}

// Snapshot returns the persisted usage snapshot when SQLite is configured.
func snapshotFromSQLite() (StatisticsSnapshot, bool) {
	store := currentSQLiteStore()
	if store == nil {
		return StatisticsSnapshot{}, false
	}
	// Keep management reads bounded. The async writer commits every batch quickly,
	// so a snapshot may lag by one batch instead of waiting for a busy queue to drain.
	snapshot, errSnapshot := store.snapshot(time.Now())
	if errSnapshot != nil {
		log.WithError(errSnapshot).Warn("failed to read token usage SQLite database")
		return StatisticsSnapshot{}, false
	}
	return snapshot, true
}
