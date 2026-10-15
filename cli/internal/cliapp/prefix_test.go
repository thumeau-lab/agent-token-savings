package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPrefixJSONReportsClaudeCodeComponents(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "CLAUDE.md"), []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "--json"}, &out, &errOut, Env{HomeDir: home, WorkDir: cwd})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Command string `json:"command"`
		Agents  []struct {
			Agent           string `json:"agent"`
			Skipped         bool   `json:"skipped"`
			TotalBytes      int64  `json:"total_bytes,omitempty"`
			EstimatedTokens int64  `json:"estimated_tokens,omitempty"`
			Components      []struct {
				Name  string `json:"name"`
				Bytes int64  `json:"bytes"`
			} `json:"components,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}

	var claude *struct {
		Agent           string `json:"agent"`
		Skipped         bool   `json:"skipped"`
		TotalBytes      int64  `json:"total_bytes,omitempty"`
		EstimatedTokens int64  `json:"estimated_tokens,omitempty"`
		Components      []struct {
			Name  string `json:"name"`
			Bytes int64  `json:"bytes"`
		} `json:"components,omitempty"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "claude-code" {
			claude = &payload.Agents[i]
		}
	}
	if claude == nil {
		t.Fatalf("no claude-code entry in %+v", payload.Agents)
	}
	if claude.TotalBytes != 8 || claude.EstimatedTokens != 2 {
		t.Fatalf("claude-code totals = %+v, want bytes=8 tokens=2", claude)
	}
}
