package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPrefixReportsClaudeCodeBuiltinEvenWithoutLocalFiles verifies that
// claude-code is never fully skipped just because no local memory/skill/MCP
// file exists: it always has a non-zero built-in (not measurable offline)
// figure, per the card's "never as 0" requirement.
func TestPrefixReportsClaudeCodeBuiltinEvenWithoutLocalFiles(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir() // empty: no CLAUDE.md, no skills, no MCP config

	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "--json"}, &out, &errOut, Env{HomeDir: home, WorkDir: cwd})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent         string `json:"agent"`
			Skipped       bool   `json:"skipped"`
			BuiltinTokens int64  `json:"builtin_tokens,omitempty"`
			BuiltinSource string `json:"builtin_source,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	var claude *struct {
		Agent         string `json:"agent"`
		Skipped       bool   `json:"skipped"`
		BuiltinTokens int64  `json:"builtin_tokens,omitempty"`
		BuiltinSource string `json:"builtin_source,omitempty"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "claude-code" {
			claude = &payload.Agents[i]
		}
	}
	if claude == nil {
		t.Fatalf("no claude-code entry in %+v", payload.Agents)
	}
	if claude.Skipped {
		t.Fatalf("claude-code marked skipped despite always having a built-in figure: %+v", claude)
	}
	if claude.BuiltinTokens <= 0 || claude.BuiltinSource == "" {
		t.Fatalf("claude-code builtin_tokens/_source = %d/%q, want > 0 / non-empty", claude.BuiltinTokens, claude.BuiltinSource)
	}
}

// TestPrefixReportsHermesComponents verifies `ats prefix hermes --json`
// is wired to prefixinfo.AnalyzeHermes (memory files, skills index,
// config.yaml split) instead of the old blanket skip.
func TestPrefixReportsHermesComponents(t *testing.T) {
	home := t.TempDir()
	hermesHome := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesHome, "SOUL.md"), []byte("identity"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "--json", "hermes"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	var payload struct {
		Agents []struct {
			Agent      string `json:"agent"`
			Skipped    bool   `json:"skipped"`
			Components []struct {
				Name  string `json:"name"`
				Bytes int64  `json:"bytes"`
			} `json:"components,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 || payload.Agents[0].Skipped {
		t.Fatalf("hermes agent = %+v, want one non-skipped entry", payload.Agents)
	}
	var found bool
	for _, c := range payload.Agents[0].Components {
		if c.Name == "SOUL.md" && c.Bytes == int64(len("identity")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no SOUL.md component in %+v", payload.Agents[0].Components)
	}
}
