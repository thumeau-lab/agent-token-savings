package cliapp

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

func writeHermesConfig(t *testing.T, home, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFixDryRunNeverChangesTheFile(t *testing.T) {
	home := t.TempDir()
	configPath := writeHermesConfig(t, home, "prompt_caching:\n  cache_ttl: 1h\n")
	before, _ := os.ReadFile(configPath)
	beforeSHA := sha256.Sum256(before)

	var out, errOut bytes.Buffer
	code := Run([]string{"fix"}, &out, &errOut, Env{HomeDir: home, IsTTY: false})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "cache_ttl") {
		t.Fatalf("output does not mention cache_ttl: %s", out.String())
	}

	after, _ := os.ReadFile(configPath)
	afterSHA := sha256.Sum256(after)
	if beforeSHA != afterSHA {
		t.Fatalf("ats fix (dry run) modified the file: before sha=%x after sha=%x", beforeSHA, afterSHA)
	}
}

func TestFixWriteCreatesBackupAndUpdatesValue(t *testing.T) {
	home := t.TempDir()
	configPath := writeHermesConfig(t, home, "prompt_caching:\n  cache_ttl: 1h\n")

	var out, errOut bytes.Buffer
	code := Run([]string{"fix", "--write"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}

	newContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newContent), "cache_ttl: auto") {
		t.Fatalf("config not updated: %q", newContent)
	}

	matches, _ := filepath.Glob(configPath + ".bak-*")
	if len(matches) != 1 {
		t.Fatalf("backup files = %v, want exactly 1", matches)
	}
	backupContent, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(backupContent), "cache_ttl: 1h") {
		t.Fatalf("backup content lost the old value: %q", backupContent)
	}
}

func TestFixNothingToFixWhenAlreadyAuto(t *testing.T) {
	home := t.TempDir()
	writeHermesConfig(t, home, "prompt_caching:\n  cache_ttl: auto\n")

	var out, errOut bytes.Buffer
	code := Run([]string{"fix"}, &out, &errOut, Env{HomeDir: home, Lang: i18n.EN})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "nothing to fix") {
		t.Fatalf("output = %q, want a 'nothing to fix' message", out.String())
	}
}
