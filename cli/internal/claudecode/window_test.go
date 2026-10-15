package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAggregateFiltersRowsOutsideDaysWindow is the regression test for
// the bug where Aggregate ignored --days entirely: `ats cache --days
// 1`, `--days 7` and `--days 30` all reported the exact same all-time
// totals. A single session file carries one row 2 days before now
// (inside a 7-day window) and one row 40 days before now (outside it)
// -- only the first must be counted.
func TestAggregateFiltersRowsOutsideDaysWindow(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	recent := now.AddDate(0, 0, -2).Format(time.RFC3339)
	old := now.AddDate(0, 0, -40).Format(time.RFC3339)

	dir := filepath.Join(root, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(
		`{"timestamp":%q,"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000}}}`+"\n"+
			`{"timestamp":%q,"message":{"usage":{"input_tokens":2,"output_tokens":6,"cache_read_input_tokens":0,"cache_creation_input_tokens":2000}}}`+"\n",
		recent, old)
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Aggregate(root, 7, now)
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if rep.Calls != 1 {
		t.Fatalf("Calls = %d, want 1 (only the row 2 days old is inside a 7-day window)", rep.Calls)
	}
	if rep.Totals.Create != 1000 {
		t.Fatalf("Totals.Create = %d, want 1000 (the 40-day-old row must be excluded)", rep.Totals.Create)
	}
}

// TestAggregateKeepsRowsWithNoParsableTimestampOut pins the card's
// explicit rule: a row whose timestamp cannot be parsed can never be
// placed inside the window, so it must be dropped rather than
// defaulted into the window.
func TestAggregateKeepsRowsWithNoParsableTimestampOut(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	dir := filepath.Join(root, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No "timestamp" field at all.
	content := `{"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Aggregate(root, 7, now)
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if rep.Calls != 0 || rep.Sessions != 0 {
		t.Fatalf("Calls=%d Sessions=%d, want 0/0 (a row with no parsable timestamp must stay out of the window)", rep.Calls, rep.Sessions)
	}
}
