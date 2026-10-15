package codexcli

import (
	"path/filepath"
	"testing"
)

// TestParseSessionFileAttachesModelFromTurnContext is the card's
// acceptance test for Codex: rollout files carry a "turn_context"
// line (payload.model, see codex-rs protocol.rs TurnContextItem)
// before the token_count events of that turn. Each UsageRow must
// carry the model active at the time of its own event, not an empty
// string, so the Dashboard's "Par modele" panel can group Codex calls
// per model like every other agent.
func TestParseSessionFileAttachesModelFromTurnContextToSubsequentRows(t *testing.T) {
	dir := t.TempDir()
	content := `{"timestamp":"2026-10-14T10:00:00Z","type":"turn_context","payload":{"cwd":"/tmp","approval_policy":"never","sandbox_policy":"read-only","model":"gpt-5.1-codex"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":50,"output_tokens":30}}}}` + "\n"
	p := writeFile(t, dir, "rollout-model.jsonl", content)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows = %+v, want 1", res.Rows)
	}
	if res.Rows[0].Model != "gpt-5.1-codex" {
		t.Fatalf("Rows[0].Model = %q, want %q", res.Rows[0].Model, "gpt-5.1-codex")
	}
}

// TestParseSessionFileRowsBeforeAnyTurnContextHaveNoModel: a row that
// appears before any turn_context line (should not normally happen,
// but the parser must never invent a model) carries an empty Model.
func TestParseSessionFileRowsBeforeAnyTurnContextHaveNoModel(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":1}}}}` + "\n"
	p := writeFile(t, dir, "rollout-nomodel.jsonl", content)

	res, err := ParseSessionFile(p)
	if err != nil {
		t.Fatalf("ParseSessionFile: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0].Model != "" {
		t.Fatalf("Rows = %+v, want 1 row with empty Model", res.Rows)
	}
}

// TestAggregateGroupsModelBreakdownAcrossTurns covers the end-to-end
// Aggregate path: a model switch mid-session (two turn_context lines)
// must produce two ModelBreakdown rows, each with the calls/tokens
// that happened under that model.
func TestAggregateGroupsModelBreakdownAcrossTurns(t *testing.T) {
	root := t.TempDir()
	sessDir := filepath.Join(root, "sessions", "2026", "10", "14")
	content := `{"type":"turn_context","payload":{"model":"gpt-5.1-codex"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":50,"output_tokens":30}}}}` + "\n" +
		`{"type":"turn_context","payload":{"model":"gpt-5.1-codex-mini"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":2000,"cached_input_tokens":500,"cache_write_input_tokens":60,"output_tokens":40}}}}` + "\n"
	writeFile(t, sessDir, "rollout-switch.jsonl", content)

	rep, err := Aggregate(root)
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(rep.ModelBreakdown) != 2 {
		t.Fatalf("ModelBreakdown = %+v, want 2 groups", rep.ModelBreakdown)
	}
	var codex, mini *ModelUsage
	for i := range rep.ModelBreakdown {
		switch rep.ModelBreakdown[i].Model {
		case "gpt-5.1-codex":
			codex = &rep.ModelBreakdown[i]
		case "gpt-5.1-codex-mini":
			mini = &rep.ModelBreakdown[i]
		}
	}
	if codex == nil || codex.Calls != 1 || codex.Totals.CachedInput != 200 {
		t.Fatalf("codex group = %+v, want calls=1 cached_input=200", codex)
	}
	if mini == nil || mini.Calls != 1 || mini.Totals.CachedInput != 500 {
		t.Fatalf("mini group = %+v, want calls=1 cached_input=500", mini)
	}
}
