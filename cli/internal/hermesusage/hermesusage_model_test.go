package hermesusage

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// buildModelFixtureDB writes a state.db with the real `model` column
// (sessions table, schema v1+, see Hermes Agent's session-storage.md)
// and two sessions on different models, plus one session with a NULL
// model (older/edge-case row): the card's "Hermes (state.db sessions
// carry the model)" fix must group by that column, never invent a
// model name for a NULL row.
func buildModelFixtureDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE sessions (id TEXT, source TEXT, started_at REAL, model TEXT,
		api_call_count INT, input_tokens INT, output_tokens INT, cache_read_tokens INT, cache_write_tokens INT)`); err != nil {
		t.Fatal(err)
	}
	now := float64(time.Now().Unix())
	rows := [][]any{
		{"s1", "cli", now, "opus-5.5", 10, 1_000, 50, 2_000_000, 100_000},
		{"s2", "cli", now, "sonnet-5", 20, 2_000, 60, 500_000, 50_000},
		{"s3", "cli", now, nil, 5, 100, 10, 0, 0}, // no model: must not be invented
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO sessions VALUES (?,?,?,?,?,?,?,?,?)`, r...); err != nil {
			t.Fatal(err)
		}
	}
	return dbPath
}

// TestQueryByModelGroupsSessionsByModel is the card's acceptance test
// for Hermes: sessions carry a `model` column, so the Dashboard's "Par
// modele" panel must aggregate it per model instead of showing no
// per-model detail at all.
func TestQueryByModelGroupsSessionsByModel(t *testing.T) {
	dbPath := buildModelFixtureDB(t)

	rows, err := QueryByModel(dbPath, 7)
	if err != nil {
		t.Fatalf("QueryByModel: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 (the NULL-model session excluded)", rows)
	}
	var opus, sonnet *ModelUsage
	for i := range rows {
		switch rows[i].Model {
		case "opus-5.5":
			opus = &rows[i]
		case "sonnet-5":
			sonnet = &rows[i]
		}
	}
	if opus == nil || opus.Sessions != 1 || opus.Calls != 10 || opus.CacheRead != 2_000_000 || opus.CacheWrite != 100_000 {
		t.Fatalf("opus row = %+v, want sessions=1 calls=10 cache_read=2000000 cache_write=100000", opus)
	}
	if sonnet == nil || sonnet.Sessions != 1 || sonnet.Calls != 20 {
		t.Fatalf("sonnet row = %+v, want sessions=1 calls=20", sonnet)
	}
}

// TestModelUsageHitRateFormula pins ModelUsage.HitRate(): read /
// (read + write), the same formula every other agent's per-model row
// uses, 0 when there is nothing to divide.
func TestModelUsageHitRateFormula(t *testing.T) {
	m := ModelUsage{CacheRead: 300, CacheWrite: 100}
	if got, want := m.HitRate(), 0.75; got != want {
		t.Fatalf("HitRate() = %v, want %v", got, want)
	}
	var empty ModelUsage
	if got := empty.HitRate(); got != 0 {
		t.Fatalf("HitRate() on empty ModelUsage = %v, want 0", got)
	}
}
