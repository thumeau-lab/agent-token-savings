package hermesusage

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func buildFixtureDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE sessions (id TEXT, source TEXT, started_at REAL, api_call_count INT,
		 input_tokens INT, output_tokens INT, cache_read_tokens INT, cache_write_tokens INT)`)
	if err != nil {
		t.Fatal(err)
	}
	now := float64(time.Now().Unix())
	_, err = db.Exec(`INSERT INTO sessions VALUES (?,?,?,?,?,?,?,?)`,
		"old", "cli", now-5*86400, 10, 1, 50, 2_000_000, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO sessions VALUES (?,?,?,?,?,?,?,?)`,
		"new", "cli", now, 10, 1, 50, 500_000, 10)
	if err != nil {
		t.Fatal(err)
	}
	return dbPath
}

// TestQueryDailyMatchesPythonOnFixtureDB is the golden test required by the
// card: tools/hermes_usage.py --json and the Go port must report the same
// per-day numbers for the same fixture state.db.
func TestQueryDailyMatchesPythonOnFixtureDB(t *testing.T) {
	dbPath := buildFixtureDB(t)

	rows, err := QueryDaily(dbPath, 7)
	if err != nil {
		t.Fatalf("QueryDaily: %v", err)
	}

	outJSON := filepath.Join(t.TempDir(), "out.json")
	pyScript := filepath.Join(repoRoot(t), "tools", "hermes_usage.py")
	out, err := exec.Command("python3", pyScript, "--db", dbPath, "--days", "7", "--json", outJSON).CombinedOutput()
	if err != nil {
		t.Fatalf("python3 %s: %v\n%s", pyScript, err, out)
	}

	raw, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatalf("reading python json output: %v", err)
	}
	var py struct {
		Days []struct {
			Day        string `json:"day"`
			Sessions   int64  `json:"sessions"`
			Calls      int64  `json:"calls"`
			Output     int64  `json:"output"`
			Input      int64  `json:"input"`
			CacheRead  int64  `json:"cache_read"`
			CacheWrite int64  `json:"cache_write"`
		} `json:"days"`
	}
	if err := json.Unmarshal(raw, &py); err != nil {
		t.Fatalf("unmarshal python json: %v", err)
	}

	if len(py.Days) != len(rows) {
		t.Fatalf("day count: python=%d go=%d", len(py.Days), len(rows))
	}
	for i, pd := range py.Days {
		g := rows[i]
		if pd.Day != g.Day || pd.Sessions != g.Sessions || pd.Calls != g.Calls ||
			pd.Output != g.Output || pd.Input != g.Input || pd.CacheRead != g.CacheRead || pd.CacheWrite != g.CacheWrite {
			t.Errorf("day %d: python=%+v go=%+v", i, pd, g)
		}
	}
}

func TestSummarizeWeightedCostFlipsTheRanking(t *testing.T) {
	// 1M read, 100k write, 10k output: reads dominate in tokens, writes in dollars.
	s := Summarize([]DailyRow{{Day: "2026-10-01", Sessions: 1, Calls: 10, Output: 10_000, CacheRead: 1_000_000, CacheWrite: 100_000}})
	if s.ReadOverOutput != 100 {
		t.Errorf("ReadOverOutput = %v, want 100", s.ReadOverOutput)
	}
	if math.Abs(s.USD["cache_write"]-0.8) > 1e-9 || math.Abs(s.USD["cache_read"]-0.2) > 1e-9 {
		t.Errorf("USD = %+v, want write=0.8 read=0.2", s.USD)
	}
	best := ""
	for k, v := range s.USDShare {
		if best == "" || v > s.USDShare[best] {
			best = k
		}
	}
	if best != "cache_write" {
		t.Errorf("biggest USD share = %q, want cache_write", best)
	}
	if s.WritePerCall != 10_000 {
		t.Errorf("WritePerCall = %d, want 10000", s.WritePerCall)
	}
}
