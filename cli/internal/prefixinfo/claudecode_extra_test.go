package prefixinfo

import (
	"path/filepath"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
)

// TestAnalyzeClaudeCodeReportsBuiltinNotMeasurableOffline verifies the
// card's requirement: the built-in system prompt + tool schemas, which
// cannot be sized from local files, must show as an explicit figure (the
// repo's own measured number from bench/claude_code_levers.py /
// docs/levers.md) and never silently read as 0.
func TestAnalyzeClaudeCodeReportsBuiltinNotMeasurableOffline(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	if rep.BuiltinTokens <= 0 {
		t.Fatalf("BuiltinTokens = %d, want > 0 (never silently 0)", rep.BuiltinTokens)
	}
	if rep.BuiltinSource == "" {
		t.Fatalf("BuiltinSource is empty, want a citation of the measured figure")
	}
}

// TestAnalyzeClaudeCodeSumsAgentsFrontmatter verifies that custom subagent
// definitions (.claude/agents/*.md) contribute their frontmatter
// name+description, the same way skills do, since the card asks for
// "skills and agents frontmatter".
func TestAnalyzeClaudeCodeSumsAgentsFrontmatter(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	agentMD := "---\n" +
		"name: reviewer\n" +
		"description: Reviews pull requests.\n" +
		"---\n" +
		"# Body\nNot counted.\n"
	writeFile(t, filepath.Join(cwd, ".claude", "agents", "reviewer.md"), agentMD)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	var got *Component
	for i := range rep.Components {
		if rep.Components[i].Name == "agents index" {
			got = &rep.Components[i]
		}
	}
	if got == nil {
		t.Fatalf("no 'agents index' component in %+v", rep.Components)
	}
	want := int64(len("name: reviewer\ndescription: Reviews pull requests.\n"))
	if got.Bytes != want {
		t.Fatalf("agents index bytes = %d, want %d", got.Bytes, want)
	}
}

// TestAnalyzeClaudeCodeReadsMCPServersFromClaudeJSON verifies that MCP
// servers configured in the global ~/.claude.json (same mcpServers shape
// as .mcp.json) are counted too, not just the project-level file.
func TestAnalyzeClaudeCodeReadsMCPServersFromClaudeJSON(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, ".claude.json"),
		`{"mcpServers":{"global-demo":{"command":"demo-server"}}}`)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	var found bool
	for _, c := range rep.Components {
		if c.Name == "mcp: global-demo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no 'mcp: global-demo' component in %+v", rep.Components)
	}
}
