package cliapp

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"

	_ "modernc.org/sqlite"
)

// writeCodexFixture writes one rollout file under
// home/.codex/sessions/2026/10/14/, with: a normal token_count event, a
// consecutive duplicate (must be deduped), and a null-info event
// (interactive-only, counted as "without usage"). No
// cache_write_input_tokens field at all, exercising the n/a path.
func writeCodexFixture(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "14")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	usage := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":30,"reasoning_output_tokens":5,"total_tokens":1030}}}}`
	content := usage + "\n" + usage + "\n" // consecutive duplicate deduped
	if err := os.WriteFile(filepath.Join(dir, "rollout-a.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	nullInfo := `{"type":"event_msg","payload":{"type":"token_count","info":null}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-interactive.jsonl"), []byte(nullInfo), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCacheJSONReportsCodexFromRolloutFixture is the acceptance test for
// wiring codexcli into `ats cache`: real rollout files produce a
// non-skipped entry with cache-inclusive-input subtraction, a "never 0"
// n/a cache write, and the interactive-only session counted (not dropped).
func TestCacheJSONReportsCodexFromRolloutFixture(t *testing.T) {
	home := t.TempDir()
	writeCodexFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent                string `json:"agent"`
			Skipped               bool   `json:"skipped"`
			Sessions              int64  `json:"sessions"`
			Calls                 int64  `json:"calls"`
			CacheReadTokens       int64  `json:"cache_read_tokens"`
			InputTokens           int64  `json:"input_tokens"`
			CacheWriteUnknown     bool   `json:"cache_write_unknown"`
			SessionsWithoutUsage  int64  `json:"sessions_without_usage"`
			CostUnknown           bool   `json:"cost_unknown"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1 (codex only)", payload.Agents)
	}
	codex := payload.Agents[0]
	if codex.Skipped {
		t.Fatalf("codex.Skipped = true, want false (rollout fixture present)")
	}
	if codex.Sessions != 1 || codex.Calls != 1 {
		t.Fatalf("codex = %+v, want sessions=1 calls=1 (duplicate deduped)", codex)
	}
	if codex.CacheReadTokens != 200 || codex.InputTokens != 800 {
		t.Fatalf("codex = %+v, want cache_read=200 input=800 (1000-200, cache-inclusive subtraction)", codex)
	}
	if !codex.CacheWriteUnknown {
		t.Fatalf("codex.CacheWriteUnknown = false, want true (fixture has no cache_write_input_tokens field)")
	}
	if codex.SessionsWithoutUsage != 1 {
		t.Fatalf("codex.SessionsWithoutUsage = %d, want 1 (the interactive-only session is counted)", codex.SessionsWithoutUsage)
	}
	if !codex.CostUnknown {
		t.Fatalf("codex.CostUnknown = false, want true (no pricing table entry for codex models)")
	}
}

// writeCopilotFixture builds a Copilot session-store.db with events
// across two models (gpt-5, claude-sonnet-4.5), exercising the per-model
// grouping the card requires.
func writeCopilotFixture(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".copilot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "session-store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE assistant_usage_events (
		session_id TEXT, model TEXT, input_tokens INTEGER, output_tokens INTEGER,
		cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER, created_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	rows := [][]any{
		{"s1", "gpt-5", 1000, 50, 200, 30, 10, "2026-10-14T11:00:00.000Z"},
		{"s2", "claude-sonnet-4.5", 500, 25, 0, 0, 0, "2026-10-14T11:10:00.000Z"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO assistant_usage_events
			(session_id, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, r...); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCacheJSONReportsCopilotGroupedByModel is the acceptance test for
// wiring copilotcli into `ats cache`: a model switch must produce a
// per-model breakdown, not one blended total.
func TestCacheJSONReportsCopilotGroupedByModel(t *testing.T) {
	home := t.TempDir()
	writeCopilotFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "copilot", "--json", "--days", "30"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent          string  `json:"agent"`
			Skipped        bool    `json:"skipped"`
			CostUnknown    bool    `json:"cost_unknown"`
			ModelBreakdown []struct {
				Model           string  `json:"model"`
				InputTokens     int64   `json:"input_tokens"`
				CacheReadTokens int64   `json:"cache_read_tokens"`
				HitRate         float64 `json:"hit_rate"`
			} `json:"model_breakdown"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Skipped {
		t.Fatalf("copilot = %+v, want one non-skipped entry", payload.Agents)
	}
	copilot := payload.Agents[0]
	if !copilot.CostUnknown {
		t.Fatalf("copilot.CostUnknown = false, want true (no pricing table entry)")
	}
	if len(copilot.ModelBreakdown) != 2 {
		t.Fatalf("model_breakdown = %+v, want 2 groups (gpt-5, claude-sonnet-4.5)", copilot.ModelBreakdown)
	}
	var gpt5 *struct {
		Model           string  `json:"model"`
		InputTokens     int64   `json:"input_tokens"`
		CacheReadTokens int64   `json:"cache_read_tokens"`
		HitRate         float64 `json:"hit_rate"`
	}
	for i := range copilot.ModelBreakdown {
		if copilot.ModelBreakdown[i].Model == "gpt-5" {
			gpt5 = &copilot.ModelBreakdown[i]
		}
	}
	if gpt5 == nil {
		t.Fatalf("no gpt-5 group in %+v", copilot.ModelBreakdown)
	}
	if gpt5.InputTokens != 800 || gpt5.CacheReadTokens != 200 {
		t.Fatalf("gpt5 = %+v, want input=800 (1000-200) read=200", gpt5)
	}
}

// TestCacheTextCodexShowsUnknownPriceAndNAWriteEnAndFr is the golden
// card acceptance test (en+fr) for codex: "unknown price" and "n/a"
// cache write must never render as a bare number or an untranslated
// English fragment in French.
func TestCacheTextCodexShowsUnknownPriceAndNAWriteEnAndFr(t *testing.T) {
	home := t.TempDir()
	writeCodexFixture(t, home)

	for _, lang := range []i18n.Lang{i18n.EN, i18n.FR} {
		var out, errOut bytes.Buffer
		code := Run([]string{"cache", "codex"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: lang})
		if code != 0 {
			t.Fatalf("[%s] exit code = %d, want 0 (stderr: %s)", lang, code, errOut.String())
		}
		text := out.String()
		if lang == i18n.EN {
			if !strings.Contains(text, "unknown price") {
				t.Fatalf("[en] card missing \"unknown price\":\n%s", text)
			}
			if !strings.Contains(text, "n/a") {
				t.Fatalf("[en] card missing \"n/a\" for cache write:\n%s", text)
			}
		} else {
			if !strings.Contains(text, "prix inconnu") {
				t.Fatalf("[fr] card missing \"prix inconnu\":\n%s", text)
			}
			if !strings.Contains(text, "n/d") {
				t.Fatalf("[fr] card missing \"n/d\" for cache write:\n%s", text)
			}
			if strings.Contains(text, "unknown price") || strings.Contains(text, "Sessions without usage") {
				t.Fatalf("[fr] card leaks English text:\n%s", text)
			}
		}
	}
}

// TestCacheTextCopilotShowsModelBreakdownEnAndFr is the golden card
// acceptance test (en+fr) for copilot's per-model table.
func TestCacheTextCopilotShowsModelBreakdownEnAndFr(t *testing.T) {
	home := t.TempDir()
	writeCopilotFixture(t, home)

	for _, lang := range []i18n.Lang{i18n.EN, i18n.FR} {
		var out, errOut bytes.Buffer
		code := Run([]string{"cache", "copilot", "--days", "30"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: lang})
		if code != 0 {
			t.Fatalf("[%s] exit code = %d, want 0 (stderr: %s)", lang, code, errOut.String())
		}
		text := out.String()
		if !strings.Contains(text, "gpt-5") || !strings.Contains(text, "claude-sonnet-4.5") {
			t.Fatalf("[%s] card missing per-model rows:\n%s", lang, text)
		}
		if lang == i18n.FR && strings.Contains(text, "unknown price") {
			t.Fatalf("[fr] card leaks English \"unknown price\":\n%s", text)
		}
	}
}
