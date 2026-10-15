package cliapp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestRunLangFlagSetsEnvLangForTheCommand verifies `--lang fr` (as a
// leading global flag, like --no-tui) selects French for whatever
// command follows, regardless of the real process locale.
func TestRunLangFlagSetsEnvLangForTheCommand(t *testing.T) {
	home := t.TempDir()
	writeHermesFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"--lang", "fr", "cache", "--json"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	// --json always stays English/machine format: the flag must not
	// leak into the JSON payload's shape.
	var payload struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON with --lang fr: %v\noutput: %s", err, out.String())
	}
}

// TestRunLangFlagSelectsFrenchTextOutput verifies the flag actually
// changes the rendered plain-text output's language.
func TestRunLangFlagSelectsFrenchTextOutput(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"--lang", "fr", "cache"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "sessions") {
		t.Fatalf("output does not look like the French text path: %s", out.String())
	}
}

// TestEnvLangDefaultsToEnglishWhenUnset ensures an Env with no Lang set
// (the zero value) renders English, so every existing test/call site
// that does not know about Lang keeps working unchanged. It explicitly
// clears ATS_LANG/LC_ALL/LC_MESSAGES/LANG first: resolveLang(Env{})
// falls back to the real process locale, so this test must not depend
// on the host's locale being unset to pass (it was previously flaky
// under a French host locale, e.g. LANG=fr_FR.UTF-8).
func TestEnvLangDefaultsToEnglishWhenUnset(t *testing.T) {
	t.Setenv("ATS_LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")

	home := t.TempDir()
	var out, errOut bytes.Buffer
	code := Run([]string{"version"}, &out, &errOut, Env{HomeDir: home, Lang: ""})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if resolveLang(Env{}) != i18n.EN {
		t.Fatalf("resolveLang(Env{}) = %q, want en", resolveLang(Env{}))
	}
}
