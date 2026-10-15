package claudecode

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProjectJSONL(t *testing.T, root, project, name string, lines []string) {
	t.Helper()
	dir := filepath.Join(root, "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDailyUsageGroupsAcrossSessionsByCalendarDay is the tracer-bullet
// test for the Cache panel's per-day sparkline: two calls on the same
// day across two different session files must land in the same bucket,
// and a call on a later day must open a new one. DailyUsage reuses
// ParseSessionFile (the exact parser Aggregate already uses) -- no
// second parser, just a different grouping of the same rows.
func TestDailyUsageGroupsAcrossSessionsByCalendarDay(t *testing.T) {
	root := t.TempDir()
	writeProjectJSONL(t, root, "p1", "a.jsonl", []string{
		`{"timestamp":"2026-10-11T10:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":1000,"cache_creation_input_tokens":0}}}`,
	})
	writeProjectJSONL(t, root, "p1", "b.jsonl", []string{
		`{"timestamp":"2026-10-11T14:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":2000,"cache_creation_input_tokens":500}}}`,
	})
	writeProjectJSONL(t, root, "p1", "c.jsonl", []string{
		`{"timestamp":"2026-10-12T09:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":9000}}}`,
	})

	rows, err := DailyUsage(root)
	if err != nil {
		t.Fatalf("DailyUsage: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 calendar days, got %+v", len(rows), rows)
	}
	if rows[0].Day != "2026-10-11" || rows[1].Day != "2026-10-12" {
		t.Fatalf("days = [%q %q], want [2026-10-11 2026-10-12] in order", rows[0].Day, rows[1].Day)
	}
	if rows[0].Read != 3000 || rows[0].Create != 500 {
		t.Fatalf("2026-10-11 bucket = %+v, want Read=3000 Create=500", rows[0])
	}
	if rows[1].Read != 0 || rows[1].Create != 9000 {
		t.Fatalf("2026-10-12 bucket = %+v, want Read=0 Create=9000", rows[1])
	}
}

// TestDailyUsageSkipsRowsWithNoTimestamp verifies a row with an empty
// timestamp (absent in some fixtures/older transcripts) is dropped from
// the per-day history instead of corrupting a bucket under an empty key.
func TestDailyUsageSkipsRowsWithNoTimestamp(t *testing.T) {
	root := t.TempDir()
	writeProjectJSONL(t, root, "p1", "a.jsonl", []string{
		`{"message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":1000}}}`,
	})

	rows, err := DailyUsage(root)
	if err != nil {
		t.Fatalf("DailyUsage: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("len(rows) = %d, want 0 (no timestamp to bucket by)", len(rows))
	}
}
