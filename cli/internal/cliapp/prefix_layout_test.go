package cliapp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrefixAcceptsDaysFlagWithoutErroring is the regression test for
// "every command accepts --days (prefix ignores it silently)": passing
// --days to prefix must never error, even though prefix is a static
// snapshot and does not use it.
func TestPrefixAcceptsDaysFlagWithoutErroring(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "--days", "5", "--json"}, &out, &errOut, Env{HomeDir: home, WorkDir: cwd})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
}

// TestFixAcceptsDaysFlagWithoutErroring: same regression, for `ats fix`.
func TestFixAcceptsDaysFlagWithoutErroring(t *testing.T) {
	home := t.TempDir()
	var out, errOut bytes.Buffer
	code := Run([]string{"fix", "--days", "5"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
}

// TestPrefixJSONSaysZeroMCPServersConfiguredExplicitly verifies the card's
// explicit wording requirement when no MCP server is configured anywhere.
func TestPrefixJSONSaysZeroMCPServersConfiguredExplicitly(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "--json", "claude-code"}, &out, &errOut, Env{HomeDir: home, WorkDir: cwd})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []struct {
			Agent          string `json:"agent"`
			MCPServerCount int    `json:"mcp_server_count"`
			MCPServersNote string `json:"mcp_servers_note,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents = %+v, want 1", payload.Agents)
	}
	a := payload.Agents[0]
	if a.MCPServerCount != 0 {
		t.Fatalf("mcp_server_count = %d, want 0", a.MCPServerCount)
	}
	if a.MCPServersNote != "0 MCP servers configured" {
		t.Fatalf("mcp_servers_note = %q, want %q", a.MCPServersNote, "0 MCP servers configured")
	}
}

// TestPrefixJSONListsMCPServerNamesWhenConfigured verifies the note
// includes the names when servers are configured, and the count matches.
func TestPrefixJSONListsMCPServerNamesWhenConfigured(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".mcp.json"), []byte(`{"mcpServers":{"demo-server":{"command":"x"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "--json", "claude-code"}, &out, &errOut, Env{HomeDir: home, WorkDir: cwd})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	var payload struct {
		Agents []struct {
			Agent          string `json:"agent"`
			MCPServerCount int    `json:"mcp_server_count"`
			MCPServersNote string `json:"mcp_servers_note,omitempty"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	a := payload.Agents[0]
	if a.MCPServerCount != 1 {
		t.Fatalf("mcp_server_count = %d, want 1", a.MCPServerCount)
	}
	if !strings.Contains(a.MCPServersNote, "demo-server") {
		t.Fatalf("mcp_servers_note = %q, want it to mention demo-server", a.MCPServersNote)
	}
}

// TestPrefixColoredCardUsesGluedByteUnitsAndDoesNotWrapLongLabels is the
// regression test for the prefix card layout bugs: a long component
// label (hermes' renamed config.yaml rows) must appear whole on one
// line, and byte annotations must use the glued "KB" unit, never
// "K B" split across a wrap. It asserts the English label text, so it
// pins ATS_LANG=en: without it the test depends on the host's locale
// (it was flaky under LANG=fr_FR.UTF-8, where the labels render in
// French and the English substring is never found).
func TestPrefixColoredCardUsesGluedByteUnitsAndDoesNotWrapLongLabels(t *testing.T) {
	t.Setenv("ATS_LANG", "en")
	home := t.TempDir()
	hermesHome := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "model:\n  default: opus\n" +
		"mcp_servers:\n  time:\n    command: \"uvx\"\n    args: [\"mcp-server-time\"]\n" +
		"display:\n  skin: default\n"
	if err := os.WriteFile(filepath.Join(hermesHome, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesHome, "SOUL.md"), []byte(strings.Repeat("x", 30_000)), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"prefix", "hermes"}, &out, &errOut, Env{HomeDir: home, IsTTY: true})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	text := out.String()
	if strings.Contains(text, "config.yaml (mcp_servers)") || strings.Contains(text, "config.yaml (toolsets") {
		t.Fatalf("card still uses the old colliding config.yaml label names:\n%s", text)
	}
	lines := strings.Split(text, "\n")
	foundMCP, foundToolsets := false, false
	for _, line := range lines {
		if strings.Contains(line, "MCP servers (config.yaml)") {
			foundMCP = true
		}
		if strings.Contains(line, "toolsets (config.yaml)") {
			foundToolsets = true
		}
	}
	if !foundMCP {
		t.Fatalf("label \"MCP servers (config.yaml)\" not found whole on one line:\n%s", text)
	}
	if !foundToolsets {
		t.Fatalf("label \"toolsets (config.yaml)\" not found whole on one line:\n%s", text)
	}
	if strings.Contains(text, " K B") || strings.Contains(text, " M B") {
		t.Fatalf("card still splits the scale letter from the byte unit (\"K B\"/\"M B\"):\n%s", text)
	}
	if !strings.Contains(text, "KB") {
		t.Fatalf("card missing a glued \"KB\" byte annotation (SOUL.md is 30000 bytes):\n%s", text)
	}
}
