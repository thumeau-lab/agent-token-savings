package cliapp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrefixFrenchOutputHasNoEnglishLeftovers is the regression test for
// the French Prefix card leaking raw English text (reviewer feedback):
// the claude-code verdict's built-in citation ("tool schemas = 68% of
// the prefix", "28,362 tok vs 9,072 tok with --tools") and the
// component labels "skills index", "MCP servers (config.yaml)",
// "toolsets (config.yaml)" must all go through the catalog, not leak
// verbatim English into a French render. --json always stays English
// (see collectLang in runPrefix): that path is intentionally excluded.
func TestPrefixFrenchOutputHasNoEnglishLeftovers(t *testing.T) {
	home := t.TempDir()
	hermesHome := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(filepath.Join(hermesHome, "skills", "my-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: my-skill\ndescription: Does a thing.\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(hermesHome, "skills", "my-skill", "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	config := "model:\n  default: opus\n" +
		"mcp_servers:\n  time:\n    command: \"uvx\"\n"
	if err := os.WriteFile(filepath.Join(hermesHome, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"--lang", "fr", "prefix"}, &out, &errOut, Env{HomeDir: home, IsTTY: true})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	text := out.String()

	englishLeftovers := []string{
		"skills index",
		"MCP servers (config.yaml)",
		"toolsets (config.yaml)",
		"tool schemas = 68% of the prefix",
		"28,362 tok vs 9,072 tok with --tools",
	}
	for _, leftover := range englishLeftovers {
		if strings.Contains(text, leftover) {
			t.Errorf("French prefix output still contains English leftover %q:\n%s", leftover, text)
		}
	}
	for _, want := range []string{"index des skills", "Serveurs MCP (config.yaml)", "schémas d'outils"} {
		if !strings.Contains(text, want) {
			t.Errorf("French prefix output missing localized text %q:\n%s", want, text)
		}
	}
}

// TestPrefixJSONKeepsEnglishComponentNamesRegardlessOfLang verifies
// --json never translates (component names stay byte-identical to
// prefixinfo's own English constants), even when --lang fr is given:
// --json is a machine format, documented to always stay English.
func TestPrefixJSONKeepsEnglishComponentNamesRegardlessOfLang(t *testing.T) {
	home := t.TempDir()
	hermesHome := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesHome, "SOUL.md"), []byte("identity"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"--lang", "fr", "prefix", "--json", "hermes"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "SOUL.md") {
		t.Fatalf("--json output missing SOUL.md component: %s", out.String())
	}
}
