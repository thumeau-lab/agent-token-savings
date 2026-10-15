package prefixinfo

import (
	"path/filepath"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
)

// TestAnalyzeClaudeCodeReportsZeroMCPServersExplicitly verifies that with
// no .mcp.json, ~/.claude.json or settings.json anywhere, the report
// says so explicitly (card: "If none are configured, say '0 MCP servers
// configured' explicitly") instead of silently having an empty list.
func TestAnalyzeClaudeCodeReportsZeroMCPServersExplicitly(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	if len(rep.MCPServers) != 0 {
		t.Fatalf("MCPServers = %+v, want empty", rep.MCPServers)
	}
}

// TestAnalyzeClaudeCodeListsMCPServerNamesAcrossEverySource verifies
// MCP server names (names only, never env/args values) are collected
// from every source the card lists: project .mcp.json, project
// .claude/settings.json, user ~/.claude/settings.json and the global
// ~/.claude.json top-level mcpServers.
func TestAnalyzeClaudeCodeListsMCPServerNamesAcrossEverySource(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, ".mcp.json"), `{"mcpServers":{"project-demo":{"command":"x"}}}`)
	writeFile(t, filepath.Join(cwd, ".claude", "settings.json"), `{"mcpServers":{"project-settings-demo":{"command":"x"}}}`)
	writeFile(t, filepath.Join(roots.ClaudeHome, "settings.json"), `{"mcpServers":{"user-settings-demo":{"command":"x"}}}`)
	writeFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"global-demo":{"command":"x","env":{"SECRET":"shh"}}}}`)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	want := map[string]bool{
		"project-demo": false, "project-settings-demo": false,
		"user-settings-demo": false, "global-demo": false,
	}
	for _, name := range rep.MCPServers {
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("MCP server %q not found in %+v", name, rep.MCPServers)
		}
	}
	for _, name := range rep.MCPServers {
		if name == "" {
			continue
		}
		// Names only: no env/args value ("shh") ever leaks into the name list.
		if name == "shh" {
			t.Fatalf("MCPServers leaked an env value: %+v", rep.MCPServers)
		}
	}
}

// TestAnalyzeClaudeCodeListsPerProjectMCPServersFromClaudeJSON verifies
// the per-project "projects": {<path>: {"mcpServers": {...}}} shape in
// the global ~/.claude.json is read too, not just the top-level
// mcpServers key.
func TestAnalyzeClaudeCodeListsPerProjectMCPServersFromClaudeJSON(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, ".claude.json"),
		`{"projects":{"/some/other/project":{"mcpServers":{"per-project-demo":{"command":"x"}}}}}`)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	found := false
	for _, name := range rep.MCPServers {
		if name == "per-project-demo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("per-project-demo not found in %+v (per-project mcpServers in ~/.claude.json not read)", rep.MCPServers)
	}
}
