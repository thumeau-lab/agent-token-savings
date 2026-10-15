package cliapp

import (
	"bytes"
	"testing"
)

// TestNoTUIFlagAcceptedAfterSubcommand verifies `ats cache --no-tui`
// (the flag placed after an explicit subcommand, as the card's Done
// section invokes it) does not error: --no-tui only matters for the
// bare `ats` TUI-vs-cache decision, so once a subcommand is named
// explicitly it is a harmless no-op, not an unknown flag.
func TestNoTUIFlagAcceptedAfterSubcommand(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--no-tui"}, &out, &errOut, Env{HomeDir: home})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
}
