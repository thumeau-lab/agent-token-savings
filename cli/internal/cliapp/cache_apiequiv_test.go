package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestCacheJSONClaudeCodeHasAPIEquivalentCostNote is the regression test
// for the "Bill" line bug: Claude Code on a subscription plan is not
// billed per token, so `ats cache` must carry an explicit note alongside
// the usd/usd_share fields (kept as-is) explaining the figure is an
// API-equivalent cost, not a real per-token charge.
func TestCacheJSONClaudeCodeHasAPIEquivalentCostNote(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-10-11T09:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}` + "\n"
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
			Agent   string             `json:"agent"`
			USD     map[string]float64 `json:"usd,omitempty"`
			USDNote string             `json:"usd_note,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	var claude *struct {
		Agent   string             `json:"agent"`
		USD     map[string]float64 `json:"usd,omitempty"`
		USDNote string             `json:"usd_note,omitempty"`
	}
	for i := range payload.Agents {
		if payload.Agents[i].Agent == "claude-code" {
			claude = &payload.Agents[i]
		}
	}
	if claude == nil {
		t.Fatalf("no claude-code entry in %+v", payload.Agents)
	}
	if claude.USD == nil {
		t.Fatalf("claude-code usd field missing (JSON field name must be kept)")
	}
	if claude.USDNote == "" {
		t.Fatalf("claude-code usd_note is empty, want a note that this is not billed per token on a subscription")
	}
	if !strings.Contains(strings.ToLower(claude.USDNote), "subscription") {
		t.Fatalf("usd_note = %q, want it to mention a subscription plan is not billed per token", claude.USDNote)
	}
}

// TestCacheColoredCardLabelsAPIEquivalentCostNotBill verifies the card's
// stat label is renamed from "Bill" to "API-equivalent cost" (the JSON
// field name stays "usd" -- see usd_note test above for that).
func TestCacheColoredCardLabelsAPIEquivalentCostNotBill(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-10-11T09:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"cache"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, Lang: i18n.EN})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if strings.Contains(out.String(), "Bill") {
		t.Fatalf("card still uses the old \"Bill\" label:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "API-equivalent cost") {
		t.Fatalf("card missing \"API-equivalent cost\" label:\n%s", out.String())
	}
}

// TestCacheColoredCardShowsLast7DaysSubtitle is the regression test for
// the default-days-mismatch bug: cache and ttl must share the same
// default (7) and the card must show it explicitly as a subtitle.
func TestCacheColoredCardShowsLast7DaysSubtitle(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-10-11T09:00:00Z","message":{"usage":{"input_tokens":1,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":30000}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"cache"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, Lang: i18n.EN})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "last 7 days") {
		t.Fatalf("card missing \"last 7 days\" subtitle:\n%s", out.String())
	}
}
