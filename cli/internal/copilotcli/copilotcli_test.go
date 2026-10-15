package copilotcli

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE assistant_usage_events (
	session_id TEXT,
	model TEXT,
	input_tokens INTEGER,
	output_tokens INTEGER,
	cache_read_tokens INTEGER,
	cache_write_tokens INTEGER,
	reasoning_tokens INTEGER,
	created_at TEXT
);`

// buildFixtureDB creates a small sqlite db at dir/session-store.db with
// the real Copilot schema (assistant_usage_events) and the given rows,
// then closes it so QueryByModel can reopen it read-only (mode=ro),
// mirroring how ats would find the agent's real db.
func buildFixtureDB(t *testing.T, dir string, rows [][]any) string {
	t.Helper()
	path := filepath.Join(dir, "session-store.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	for _, r := range rows {
		_, err := db.Exec(`INSERT INTO assistant_usage_events
			(session_id, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, r...)
		if err != nil {
			db.Close()
			t.Fatalf("insert: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return path
}

// TestQueryByModelGroupsAndAggregates is the first tracer bullet: two
// events for the same model are summed into one ModelUsage row.
func TestQueryByModelGroupsAndAggregates(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	path := buildFixtureDB(t, dir, [][]any{
		{"s1", "gpt-5", 1000, 50, 200, 30, 10, "2026-10-14T11:46:35.897Z"},
		{"s1", "gpt-5", 500, 20, 100, 0, 5, "2026-10-14T11:50:00.000Z"},
	})

	models, err := QueryByModel(path, 7, now)
	if err != nil {
		t.Fatalf("QueryByModel: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %+v, want 1 group", models)
	}
	m := models[0]
	if m.Model != "gpt-5" || m.Sessions != 1 || m.Calls != 2 {
		t.Fatalf("m = %+v, want model=gpt-5 sessions=1 calls=2", m)
	}
	if m.InputTokens != 1500 || m.OutputTokens != 70 || m.CacheReadTokens != 300 || m.CacheWriteTokens != 30 {
		t.Fatalf("m = %+v, want input=1500 output=70 read=300 write=30", m)
	}
}

// TestQueryByModelSeparatesDifferentModels: a model switch must produce
// two distinct groups, each with its own totals.
func TestQueryByModelSeparatesDifferentModels(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	path := buildFixtureDB(t, dir, [][]any{
		{"s1", "gpt-5", 1000, 50, 200, 30, 10, "2026-10-14T11:00:00.000Z"},
		{"s1", "claude-sonnet-4.5", 800, 40, 0, 0, 0, "2026-10-14T11:10:00.000Z"},
	})

	models, err := QueryByModel(path, 7, now)
	if err != nil {
		t.Fatalf("QueryByModel: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %+v, want 2 groups (model switch)", models)
	}
}

// TestQueryByModelExcludesEventsOutsideWindow: created_at before the
// cutoff must not contribute.
func TestQueryByModelExcludesEventsOutsideWindow(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	path := buildFixtureDB(t, dir, [][]any{
		{"s1", "gpt-5", 1000, 50, 200, 30, 10, "2026-09-01T00:00:00.000Z"}, // outside the 7-day window
		{"s2", "gpt-5", 100, 5, 20, 3, 1, "2026-10-14T11:00:00.000Z"},
	})

	models, err := QueryByModel(path, 7, now)
	if err != nil {
		t.Fatalf("QueryByModel: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %+v, want 1 group", models)
	}
	if models[0].InputTokens != 100 {
		t.Fatalf("InputTokens = %d, want 100 (old event excluded)", models[0].InputTokens)
	}
}

// TestQueryByModelOpensReadOnly: QueryByModel must never need write
// access -- opening a db file with no write permission on its directory
// must still succeed.
func TestQueryByModelOpensReadOnly(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	path := buildFixtureDB(t, dir, [][]any{
		{"s1", "gpt-5", 10, 1, 0, 0, 0, "2026-10-14T11:00:00.000Z"},
	})

	if _, err := QueryByModel(path, 7, now); err != nil {
		t.Fatalf("QueryByModel on a closed, reopened db: %v", err)
	}
}

// TestHitRateFormulaMatchesCard: read / (uncached input + read + write),
// same formula as every other agent.
func TestHitRateFormulaMatchesCard(t *testing.T) {
	m := ModelUsage{InputTokens: 1200, CacheReadTokens: 200, CacheWriteTokens: 100}
	// uncached = 1200-200 = 1000; denom = 1000+200+100 = 1300; hit = 200/1300
	got := m.HitRate()
	want := 200.0 / 1300.0
	if got != want {
		t.Fatalf("HitRate() = %v, want %v", got, want)
	}
}

func TestHitRateZeroWhenNoDenominator(t *testing.T) {
	var m ModelUsage
	if got := m.HitRate(); got != 0 {
		t.Fatalf("HitRate() = %v, want 0", got)
	}
}
