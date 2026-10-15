package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCacheJSONComputesClaudeCodeBillShareAndWarmBreaks verifies that
// `ats cache --json` computes a USD bill share for claude-code (same
// Opus 5.5 price table as hermes, docs/measurements.md) and surfaces the
// warm-rebuild detail rows (session, time, tokens rewritten) that
// claudecode.Aggregate already computes but did not use to expose before.
func TestCacheJSONComputesClaudeCodeBillShareAndWarmBreaks(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-10-11T09:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}
{"timestamp":"2026-10-11T09:01:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":30000,"cache_creation_input_tokens":900}}}
{"timestamp":"2026-10-11T10:00:00Z","message":{"usage":{"input_tokens":2,"output_tokens":6,"cache_read_input_tokens":0,"cache_creation_input_tokens":50000}}}
`
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent              string             `json:"agent"`
			USD                map[string]float64 `json:"usd,omitempty"`
			USDShare           map[string]float64 `json:"usd_share,omitempty"`
			WarmRebuildDetails []struct {
				Session   string `json:"session"`
				Timestamp string `json:"timestamp"`
				Tokens    int64  `json:"tokens"`
			} `json:"warm_rebuild_details,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}

	var claude *struct {
		Agent              string             `json:"agent"`
		USD                map[string]float64 `json:"usd,omitempty"`
		USDShare           map[string]float64 `json:"usd_share,omitempty"`
		WarmRebuildDetails []struct {
			Session   string `json:"session"`
			Timestamp string `json:"timestamp"`
			Tokens    int64  `json:"tokens"`
		} `json:"warm_rebuild_details,omitempty"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "claude-code" {
			claude = &payload.Agents[i]
		}
	}
	if claude == nil {
		t.Fatalf("no claude-code entry in %+v", payload.Agents)
	}
	if claude.USDShare == nil || claude.USDShare["cache_write"] <= 0 {
		t.Fatalf("claude-code usd_share = %+v, want a positive cache_write share", claude.USDShare)
	}
	if len(claude.WarmRebuildDetails) != 1 {
		t.Fatalf("claude-code warm_rebuild_details = %+v, want 1 entry", claude.WarmRebuildDetails)
	}
	d := claude.WarmRebuildDetails[0]
	if d.Session != "a.jsonl" || d.Timestamp != "2026-10-11T10:00:00Z" || d.Tokens != 50000 {
		t.Fatalf("warm_rebuild_details[0] = %+v, want session=a.jsonl timestamp=2026-10-11T10:00:00Z tokens=50000", d)
	}
}
