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

func writeClaudeFixture(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	lines := []string{
		`{"timestamp":"` + ts + `","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}`,
		`{"timestamp":"` + ts + `","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":30000,"cache_creation_input_tokens":900}}}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCacheJSONReportsClaudeCodeAndSkipsAbsentAgents(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Command string `json:"command"`
		Agents  []struct {
			Agent            string `json:"agent"`
			Skipped          bool   `json:"skipped"`
			Reason           string `json:"reason,omitempty"`
			CacheWriteTokens int64  `json:"cache_write_tokens,omitempty"`
			CacheReadTokens  int64  `json:"cache_read_tokens,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if payload.Command != "cache" {
		t.Fatalf("command = %q, want cache", payload.Command)
	}

	var claude *struct {
		Agent            string `json:"agent"`
		Skipped          bool   `json:"skipped"`
		Reason           string `json:"reason,omitempty"`
		CacheWriteTokens int64  `json:"cache_write_tokens,omitempty"`
		CacheReadTokens  int64  `json:"cache_read_tokens,omitempty"`
	}
	skippedCount := 0
	for i := range payload.Agents {
		a := &payload.Agents[i]
		if a.Agent == "claude-code" {
			claude = a
		}
		if a.Skipped {
			skippedCount++
		}
	}
	if claude == nil {
		t.Fatalf("no claude-code entry in %+v", payload.Agents)
	}
	if claude.Skipped {
		t.Fatalf("claude-code marked skipped despite having fixture data: %+v", claude)
	}
	if claude.CacheWriteTokens != 30900 || claude.CacheReadTokens != 30000 {
		t.Fatalf("claude-code totals = %+v, want write=30900 read=30000", claude)
	}
	// codex, copilot, gemini-cli, hermes have no fixture data under home: skipped cleanly.
	if skippedCount != 4 {
		t.Fatalf("skippedCount = %d, want 4 (codex, copilot, gemini-cli, hermes absent)", skippedCount)
	}
}
