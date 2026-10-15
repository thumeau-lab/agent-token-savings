package cliapp

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// writeHermesFixtureWithModels writes a state.db with the real `model`
// column (sessions table) and two sessions on different models, the
// card's acceptance fixture for "Hermes (state.db sessions carry the
// model)" missing from the Dashboard's "Par modele" panel.
func writeHermesFixtureWithModels(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "state.db"))
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
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO sessions VALUES (?,?,?,?,?,?,?,?,?)`, r...); err != nil {
			t.Fatal(err)
		}
	}
}

// writeCodexFixtureWithModels writes two rollout files, each with a
// turn_context line selecting a different model before its token_count
// events, the card's acceptance fixture for "codex (rollout jsonl
// carry the model)" missing from the Dashboard's "Par modele" panel.
func writeCodexFixtureWithModels(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "14")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	one := `{"type":"turn_context","payload":{"model":"gpt-5.1-codex"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":50,"output_tokens":30}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-a.jsonl"), []byte(one), 0o644); err != nil {
		t.Fatal(err)
	}
	two := `{"type":"turn_context","payload":{"model":"gpt-5.1-codex-mini"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":500,"cached_input_tokens":100,"cache_write_input_tokens":20,"output_tokens":15}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-b.jsonl"), []byte(two), 0o644); err != nil {
		t.Fatal(err)
	}
}

type modelBreakdownAgent struct {
	Agent          string `json:"agent"`
	ModelBreakdown []struct {
		Model            string  `json:"model"`
		Calls            int64   `json:"calls"`
		CacheReadTokens  int64   `json:"cache_read_tokens"`
		CacheWriteTokens int64   `json:"cache_write_tokens"`
		HitRate          float64 `json:"hit_rate"`
	} `json:"model_breakdown"`
}

// TestCacheJSONReportsHermesGroupedByModel is the card's per-agent
// acceptance test for Hermes: `ats cache --json` must group Hermes
// sessions by their own `model` column.
func TestCacheJSONReportsHermesGroupedByModel(t *testing.T) {
	home := t.TempDir()
	writeHermesFixtureWithModels(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "hermes", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []modelBreakdownAgent `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Agent != "hermes" {
		t.Fatalf("agents = %+v, want 1 hermes entry", payload.Agents)
	}
	hermes := payload.Agents[0]
	if len(hermes.ModelBreakdown) != 2 {
		t.Fatalf("model_breakdown = %+v, want 2 groups (opus-5.5, sonnet-5)", hermes.ModelBreakdown)
	}
	var opus *struct {
		Model            string  `json:"model"`
		Calls            int64   `json:"calls"`
		CacheReadTokens  int64   `json:"cache_read_tokens"`
		CacheWriteTokens int64   `json:"cache_write_tokens"`
		HitRate          float64 `json:"hit_rate"`
	}
	for i := range hermes.ModelBreakdown {
		if hermes.ModelBreakdown[i].Model == "opus-5.5" {
			opus = &hermes.ModelBreakdown[i]
		}
	}
	if opus == nil || opus.Calls != 10 || opus.CacheReadTokens != 2_000_000 {
		t.Fatalf("opus group = %+v, want calls=10 cache_read_tokens=2000000", opus)
	}
}

// TestCacheJSONReportsCodexGroupedByModel is the card's per-agent
// acceptance test for Codex: `ats cache --json` must group Codex calls
// by the model selected in each turn_context line.
func TestCacheJSONReportsCodexGroupedByModel(t *testing.T) {
	home := t.TempDir()
	writeCodexFixtureWithModels(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "codex", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []modelBreakdownAgent `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Agent != "codex" {
		t.Fatalf("agents = %+v, want 1 codex entry", payload.Agents)
	}
	codex := payload.Agents[0]
	if len(codex.ModelBreakdown) != 2 {
		t.Fatalf("model_breakdown = %+v, want 2 groups (gpt-5.1-codex, gpt-5.1-codex-mini)", codex.ModelBreakdown)
	}
}
