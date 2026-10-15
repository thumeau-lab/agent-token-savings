package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// modelUsageLine is one jsonl line's worth of usage, tagged with the
// `message.model` value real Claude Code assistant lines carry.
type modelUsageLine struct {
	Model                        string
	Input, Output, Read, Create int64
}

// writeModelSession writes a session jsonl file with a `message.model`
// field alongside the usual `message.usage` block, to
// root/projects/<proj>/<name>.jsonl (same layout Aggregate globs).
// Every line carries a "timestamp" of the real current moment (see
// writeSession's doc comment for why).
func writeModelSession(t *testing.T, root, proj, name string, lines []modelUsageLine) {
	t.Helper()
	dir := filepath.Join(root, "projects", proj)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	var sb strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&sb, `{"timestamp":%q,"message":{"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`+"\n",
			ts, l.Model, l.Input, l.Output, l.Read, l.Create)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAggregateGroupsUsageByModelSkippingSynthetic is the card's unit
// test for per-model aggregation: a 3-line fixture covering 2 real
// models plus one "<synthetic>" line (card: "skip <synthetic> model
// rows") that must still count toward the overall Totals (never
// silently dropped from the window's real numbers, only excluded from
// the per-model breakdown).
func TestAggregateGroupsUsageByModelSkippingSynthetic(t *testing.T) {
	root := t.TempDir()
	writeModelSession(t, root, "proj", "s.jsonl", []modelUsageLine{
		{Model: "claude-opus-4-5-20251101", Input: 10, Output: 5, Read: 0, Create: 1000},
		{Model: "claude-haiku-4-5-20251101", Input: 20, Output: 8, Read: 500, Create: 0},
		{Model: "<synthetic>", Input: 1, Output: 1, Read: 0, Create: 0},
	})

	rep, err := Aggregate(root, 36600, time.Now())
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}

	if len(rep.ModelBreakdown) != 2 {
		t.Fatalf("len(ModelBreakdown) = %d, want 2 (synthetic skipped): %+v", len(rep.ModelBreakdown), rep.ModelBreakdown)
	}
	// Sorted by model name, same convention as copilotcli.QueryByModel.
	haiku, opus := rep.ModelBreakdown[0], rep.ModelBreakdown[1]
	if haiku.Model != "claude-haiku-4-5-20251101" || haiku.Calls != 1 || haiku.Totals.Read != 500 || haiku.Totals.Input != 20 {
		t.Fatalf("ModelBreakdown[0] = %+v, want claude-haiku-4-5-20251101 calls=1 read=500 input=20", haiku)
	}
	if opus.Model != "claude-opus-4-5-20251101" || opus.Calls != 1 || opus.Totals.Create != 1000 || opus.Totals.Input != 10 {
		t.Fatalf("ModelBreakdown[1] = %+v, want claude-opus-4-5-20251101 calls=1 create=1000 input=10", opus)
	}

	if rep.Totals.Input != 31 { // 10 + 20 + 1 (synthetic line still counted)
		t.Fatalf("Totals.Input = %d, want 31 (the synthetic line must still count toward the overall totals)", rep.Totals.Input)
	}
}

// TestAggregateModelBreakdownEmptyWhenNoModelField: older transcripts
// (or any line that never carried message.model) must not produce a
// blank-named group in the breakdown.
func TestAggregateModelBreakdownEmptyWhenNoModelField(t *testing.T) {
	root := t.TempDir()
	writeSession(t, root, "proj", "s.jsonl", [][4]int64{{1, 5, 0, 30000}})

	rep, err := Aggregate(root, 36600, time.Now())
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(rep.ModelBreakdown) != 0 {
		t.Fatalf("ModelBreakdown = %+v, want empty when no line carries message.model", rep.ModelBreakdown)
	}
}
