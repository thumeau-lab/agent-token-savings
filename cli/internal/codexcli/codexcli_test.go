package codexcli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestParseSessionFileReadsLastTokenUsage is the first tracer bullet: one
// event_msg/token_count line with a non-null info yields one UsageRow.
func TestParseSessionFileReadsLastTokenUsage(t *testing.T) {
	dir := t.TempDir()
	line := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":50,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":1030},"total_token_usage":{},"model_context_window":128000}}}` + "\n"
	p := writeFile(t, dir, "rollout-a.jsonl", line)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows = %+v, want 1 row", res.Rows)
	}
	row := res.Rows[0]
	if row.Input != 1000 || row.CachedInput != 200 || row.Output != 30 {
		t.Fatalf("row = %+v, want input=1000 cachedInput=200 output=30", row)
	}
	if row.CacheWrite == nil || *row.CacheWrite != 50 {
		t.Fatalf("row.CacheWrite = %v, want *50", row.CacheWrite)
	}
}

// TestParseSessionFileSkipsNullInfo: a token_count event with info=null
// (interactive TUI session, openai/codex#9660) is skipped, not an error.
func TestParseSessionFileSkipsNullInfo(t *testing.T) {
	dir := t.TempDir()
	lines := `{"type":"event_msg","payload":{"type":"token_count","info":null}}` + "\n"
	p := writeFile(t, dir, "rollout-b.jsonl", lines)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(res.Rows) != 0 {
		t.Fatalf("Rows = %+v, want 0 (null info skipped)", res.Rows)
	}
}

// TestParseSessionFileDedupesConsecutiveIdenticalUsage: codex-cli is known
// to re-emit the exact same last_token_usage on consecutive events; the
// second occurrence must not double-count.
func TestParseSessionFileDedupesConsecutiveIdenticalUsage(t *testing.T) {
	dir := t.TempDir()
	one := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":50,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":1030}}}}`
	content := one + "\n" + one + "\n"
	p := writeFile(t, dir, "rollout-c.jsonl", content)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows = %+v, want 1 (duplicate deduped)", res.Rows)
	}
}

// TestParseSessionFileReportsCacheWriteAbsentAsNilNeverZero: older
// codex-cli files lack cache_write_input_tokens entirely; the row must
// carry a nil pointer, never a 0 value (the card: "never 0").
func TestParseSessionFileReportsCacheWriteAbsentAsNilNeverZero(t *testing.T) {
	dir := t.TempDir()
	line := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":1030}}}}` + "\n"
	p := writeFile(t, dir, "rollout-d.jsonl", line)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows = %+v, want 1", res.Rows)
	}
	if res.Rows[0].CacheWrite != nil {
		t.Fatalf("CacheWrite = %v, want nil (field absent, never 0)", *res.Rows[0].CacheWrite)
	}
}

// TestParseSessionFileIgnoresTokenCountWhenTokenUsageRecordPresent: a
// newer token_usage_record event in the file means token_count must be
// ignored entirely for that file (its schema is unknown, see the TODO in
// ParseSessionFile).
func TestParseSessionFileIgnoresTokenCountWhenTokenUsageRecordPresent(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":1030}}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_usage_record","unknown_field":true}}` + "\n"
	p := writeFile(t, dir, "rollout-e.jsonl", content)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if !res.SawTokenUsageRecord {
		t.Fatalf("SawTokenUsageRecord = false, want true")
	}
	if len(res.Rows) != 0 {
		t.Fatalf("Rows = %+v, want 0 (token_count ignored once token_usage_record is seen)", res.Rows)
	}
}

// TestAggregateSumsAcrossFilesAndFlagsUnknownCacheWrite covers the
// aggregate path end to end: two session files, one with
// cache_write_input_tokens and one without, plus one file with only
// null-info events (counted as "without usage", not dropped).
func TestAggregateSumsAcrossFilesAndFlagsUnknownCacheWrite(t *testing.T) {
	root := t.TempDir()
	sessDir := filepath.Join(root, "sessions", "2026", "10", "14")
	writeFile(t, sessDir, "rollout-new.jsonl",
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":50,"output_tokens":30}}}}`+"\n")
	writeFile(t, sessDir, "rollout-old.jsonl",
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":2000,"cached_input_tokens":500,"output_tokens":60}}}}`+"\n")
	writeFile(t, sessDir, "rollout-interactive.jsonl",
		`{"type":"event_msg","payload":{"type":"token_count","info":null}}`+"\n")

	rep, err := Aggregate(root)
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if rep.Sessions != 2 {
		t.Fatalf("Sessions = %d, want 2", rep.Sessions)
	}
	if rep.SessionsWithoutUsage != 1 {
		t.Fatalf("SessionsWithoutUsage = %d, want 1 (the interactive session is counted, not dropped)", rep.SessionsWithoutUsage)
	}
	if rep.Totals.CachedInput != 700 {
		t.Fatalf("CachedInput = %d, want 700", rep.Totals.CachedInput)
	}
	if rep.Totals.CacheWrite != 50 {
		t.Fatalf("CacheWrite = %d, want 50 (only the known field)", rep.Totals.CacheWrite)
	}
	if !rep.Totals.CacheWriteUnknown {
		t.Fatalf("CacheWriteUnknown = false, want true (rollout-old.jsonl lacks the field)")
	}
	if got, want := rep.Totals.Uncached(), int64(1000-200+2000-500); got != want {
		t.Fatalf("Uncached() = %d, want %d", got, want)
	}
}

// TestAggregateFindsArchivedSessions: files under archived_sessions count too.
func TestAggregateFindsArchivedSessions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "archived_sessions"), "old-session.jsonl",
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":1}}}}`+"\n")

	rep, err := Aggregate(root)
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if rep.Sessions != 1 {
		t.Fatalf("Sessions = %d, want 1 (archived session found)", rep.Sessions)
	}
}

// TestHitRateFormulaMatchesCard: read / (uncached input + read + write).
func TestHitRateFormulaMatchesCard(t *testing.T) {
	tot := Totals{Input: 1200, CachedInput: 200, CacheWrite: 100}
	// uncached = 1200-200 = 1000; denom = 1000+200+100 = 1300; hit = 200/1300
	got := tot.HitRate()
	want := 200.0 / 1300.0
	if got != want {
		t.Fatalf("HitRate() = %v, want %v", got, want)
	}
}

func TestHitRateZeroWhenNoDenominator(t *testing.T) {
	var tot Totals
	if got := tot.HitRate(); got != 0 {
		t.Fatalf("HitRate() = %v, want 0", got)
	}
}
