package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeClaudeFixtureWithModels writes a Claude Code session jsonl with
// a `message.model` field on each line (unlike writeClaudeFixture,
// which predates per-model support), covering two real models plus a
// "<synthetic>" line that must be excluded from the model breakdown
// but still counted in the agent's own totals.
func writeClaudeFixtureWithModels(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	lines := []string{
		`{"timestamp":"` + ts + `","message":{"model":"claude-opus-4-5-20251101","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}`,
		`{"timestamp":"` + ts + `","message":{"model":"claude-haiku-4-5-20251101","usage":{"input_tokens":20,"output_tokens":8,"cache_read_input_tokens":30000,"cache_creation_input_tokens":900}}}`,
		`{"timestamp":"` + ts + `","message":{"model":"<synthetic>","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCacheJSONReportsClaudeCodeGroupedByModel is the card's acceptance
// test for wiring claudecode.Report.ModelBreakdown into `ats cache`:
// the Dashboard's "Par modele" panel must stop showing "no per-model
// detail available" once real per-model data exists.
func TestCacheJSONReportsClaudeCodeGroupedByModel(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixtureWithModels(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent          string `json:"agent"`
			ModelBreakdown []struct {
				Model            string  `json:"model"`
				Calls            int64   `json:"calls"`
				InputTokens      int64   `json:"input_tokens"`
				CacheReadTokens  int64   `json:"cache_read_tokens"`
				CacheWriteTokens int64   `json:"cache_write_tokens"`
				HitRate          float64 `json:"hit_rate"`
			} `json:"model_breakdown"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}

	var claude *struct {
		Agent          string `json:"agent"`
		ModelBreakdown []struct {
			Model            string  `json:"model"`
			Calls            int64   `json:"calls"`
			InputTokens      int64   `json:"input_tokens"`
			CacheReadTokens  int64   `json:"cache_read_tokens"`
			CacheWriteTokens int64   `json:"cache_write_tokens"`
			HitRate          float64 `json:"hit_rate"`
		} `json:"model_breakdown"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "claude-code" {
			claude = &payload.Agents[i]
		}
	}
	if claude == nil {
		t.Fatalf("no claude-code agent in %+v", payload.Agents)
	}
	if len(claude.ModelBreakdown) != 2 {
		t.Fatalf("model_breakdown = %+v, want 2 groups (synthetic excluded)", claude.ModelBreakdown)
	}
	var haiku, opus *struct {
		Model            string  `json:"model"`
		Calls            int64   `json:"calls"`
		InputTokens      int64   `json:"input_tokens"`
		CacheReadTokens  int64   `json:"cache_read_tokens"`
		CacheWriteTokens int64   `json:"cache_write_tokens"`
		HitRate          float64 `json:"hit_rate"`
	}
	for i := range claude.ModelBreakdown {
		switch claude.ModelBreakdown[i].Model {
		case "claude-haiku-4-5-20251101":
			haiku = &claude.ModelBreakdown[i]
		case "claude-opus-4-5-20251101":
			opus = &claude.ModelBreakdown[i]
		}
	}
	if haiku == nil || haiku.Calls != 1 || haiku.CacheReadTokens != 30000 {
		t.Fatalf("haiku group = %+v, want calls=1 cache_read_tokens=30000", haiku)
	}
	if opus == nil || opus.Calls != 1 || opus.CacheWriteTokens != 30000 {
		t.Fatalf("opus group = %+v, want calls=1 cache_write_tokens=30000", opus)
	}
}
