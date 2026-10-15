package prefixinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeClaudeCodeReportsMemoryFileByteSize(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	content := "# Project memory\nSome instructions.\n"
	writeFile(t, filepath.Join(cwd, "CLAUDE.md"), content)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	var found bool
	for _, c := range rep.Components {
		if c.Name == "CLAUDE.md" {
			found = true
			if c.Bytes != int64(len(content)) {
				t.Errorf("CLAUDE.md bytes = %d, want %d", c.Bytes, len(content))
			}
		}
	}
	if !found {
		t.Fatalf("no CLAUDE.md component in %+v", rep.Components)
	}
}

func TestAnalyzeClaudeCodeEstimatedTokensIsBytesOverFour(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "CLAUDE.md"), "12345678") // 8 bytes

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	if rep.TotalBytes != 8 {
		t.Fatalf("TotalBytes = %d, want 8", rep.TotalBytes)
	}
	if got := rep.EstimatedTokens(); got != 2 {
		t.Fatalf("EstimatedTokens() = %d, want 2 (8 bytes / 4)", got)
	}
}

func TestAnalyzeClaudeCodeSumsSkillNameAndDescriptionOnly(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	skill := "---\n" +
		"name: my-skill\n" +
		"description: Does a thing.\n" +
		"---\n" +
		"# Big body\nLots of unrelated text that is NOT in the skills index.\n"
	writeFile(t, filepath.Join(roots.ClaudeHome, "skills", "my-skill", "SKILL.md"), skill)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	var got *Component
	for i := range rep.Components {
		if rep.Components[i].Name == "skills index" {
			got = &rep.Components[i]
		}
	}
	if got == nil {
		t.Fatalf("no 'skills index' component in %+v", rep.Components)
	}
	want := int64(len("name: my-skill\ndescription: Does a thing.\n"))
	if got.Bytes != want {
		t.Fatalf("skills index bytes = %d, want %d (name+description only, not the body)", got.Bytes, want)
	}
}

func TestAnalyzeClaudeCodeCountsMCPServerConfigBytes(t *testing.T) {
	home := t.TempDir()
	roots, _ := agents.DefaultRoots(home)
	cwd := t.TempDir()
	mcpJSON := `{"mcpServers":{"demo":{"command":"demo-server","args":["--flag"]}}}`
	writeFile(t, filepath.Join(cwd, ".mcp.json"), mcpJSON)

	rep, err := AnalyzeClaudeCode(roots, cwd)
	if err != nil {
		t.Fatalf("AnalyzeClaudeCode: %v", err)
	}
	var got *Component
	for i := range rep.Components {
		if rep.Components[i].Name == "mcp: demo" {
			got = &rep.Components[i]
		}
	}
	if got == nil {
		t.Fatalf("no 'mcp: demo' component in %+v", rep.Components)
	}
	if got.Bytes <= 0 {
		t.Fatalf("mcp: demo bytes = %d, want > 0", got.Bytes)
	}
}
