package cliapp

import (
	"bytes"
	"testing"
)

// TestBareNonTTYFallsBackToCache is the piped/redirected case: `ats` with
// no args and IsTTY false must behave exactly as `ats cache`, unchanged
// from before the TUI existed (it must never try to start the interactive
// app without a terminal to draw into).
func TestBareNonTTYFallsBackToCache(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run(nil, &out, &errOut, Env{HomeDir: home, IsTTY: false, StdinIsTTY: false})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("bare ats piped produced no output, want the same output as `ats cache`")
	}

	var wantOut, wantErr bytes.Buffer
	wantCode := Run([]string{"cache"}, &wantOut, &wantErr, Env{HomeDir: home, IsTTY: false, StdinIsTTY: false})
	if wantCode != code || wantOut.String() != out.String() {
		t.Fatalf("bare ats (piped) output differs from `ats cache`:\nbare: %q\ncache: %q", out.String(), wantOut.String())
	}
}

// TestBareTTYWithNonTTYStdinFallsBackToCache: a redirected stdin (e.g.
// `ats < /dev/null` from a real terminal) must not start the interactive
// app either -- it needs both ends to be a TTY.
func TestBareTTYWithNonTTYStdinFallsBackToCache(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run(nil, &out, &errOut, Env{HomeDir: home, IsTTY: true, StdinIsTTY: false})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("bare ats with non-TTY stdin produced no output, want the same output as `ats cache`")
	}
}

// TestNoTUIFlagForcesCacheEvenOnATTY: `ats --no-tui` on a real terminal
// (both IsTTY and StdinIsTTY true) must still fall back to `ats cache`
// instead of starting the interactive app.
func TestNoTUIFlagForcesCacheEvenOnATTY(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"--no-tui"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, StdinIsTTY: true})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("ats --no-tui produced no output, want the same output as `ats cache`")
	}
}

// TestKnownSubcommandsUnaffectedByTUIDispatch: an explicit subcommand
// (with or without an agent/flags) must behave exactly as before,
// regardless of IsTTY/StdinIsTTY -- the TUI dispatch only ever applies to
// the fully bare invocation.
func TestKnownSubcommandsUnaffectedByTUIDispatch(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"cache", "--json"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, StdinIsTTY: true})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("ats cache --json produced no output")
	}
}
