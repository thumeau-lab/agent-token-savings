package ttlestimate

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type fixtureSession struct {
	id          string
	source      string
	writeTokens int64
	gapSeconds  int64 // largest gap between consecutive messages, 0 = single message
}

// buildFixtureHome creates a ~/.hermes-shaped directory with one state.db
// matching the schema tools/ttl_auto_estimate.py expects (sessions + messages).
func buildFixtureHome(t *testing.T, sessions []fixtureSession) string {
	t.Helper()
	home := t.TempDir()
	dbPath := filepath.Join(home, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE sessions (id TEXT, source TEXT, started_at REAL, cache_write_tokens INT)`,
		`CREATE TABLE messages (session_id TEXT, timestamp REAL)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	now := float64(time.Now().Unix())
	for _, s := range sessions {
		if _, err := db.Exec(`INSERT INTO sessions VALUES (?,?,?,?)`, s.id, s.source, now, s.writeTokens); err != nil {
			t.Fatal(err)
		}
		// two messages, gapSeconds apart, so max gap = gapSeconds.
		if _, err := db.Exec(`INSERT INTO messages VALUES (?,?)`, s.id, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO messages VALUES (?,?)`, s.id, now+float64(s.gapSeconds)); err != nil {
			t.Fatal(err)
		}
	}
	return home
}
