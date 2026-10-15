package claudecode

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAggregateWarmRebuildCapturesTimestamp verifies that each detected
// warm rebuild carries the call's own timestamp (not just session+index),
// so `ats cache`'s "worst cache breaks" table can show when it happened.
func TestAggregateWarmRebuildCapturesTimestamp(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-10-11T09:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}
{"timestamp":"2026-10-11T09:01:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":30000,"cache_creation_input_tokens":900}}}
{"timestamp":"2026-10-11T10:00:00Z","message":{"usage":{"input_tokens":2,"output_tokens":6,"cache_read_input_tokens":0,"cache_creation_input_tokens":50000}}}
`
	if err := os.WriteFile(filepath.Join(dir, "two.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Aggregate(root, 36600, time.Now())
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(rep.WarmRebuilds) != 1 {
		t.Fatalf("WarmRebuilds = %+v, want 1 entry", rep.WarmRebuilds)
	}
	got := rep.WarmRebuilds[0]
	if got.Timestamp != "2026-10-11T10:00:00Z" {
		t.Fatalf("WarmRebuilds[0].Timestamp = %q, want %q", got.Timestamp, "2026-10-11T10:00:00Z")
	}
	if got.Session != "two.jsonl" || got.Create != 50000 {
		t.Fatalf("WarmRebuilds[0] = %+v, want session=two.jsonl create=50000", got)
	}
}
