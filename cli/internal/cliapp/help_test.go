package cliapp

import (
	"bytes"
	"strings"
	"testing"
)

// TestHelpFlagPrintsUsageToStdoutAndExitsZero is the regression test
// for `ats --help`/`-h`/`help` printing "ats: unknown command" and
// exiting 2: each must print the full usage (commands, flags, env
// vars) to stdout and exit 0.
func TestHelpFlagPrintsUsageToStdoutAndExitsZero(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		var out, errOut bytes.Buffer
		code := Run(args, &out, &errOut, Env{})
		if code != 0 {
			t.Fatalf("Run(%v) exit code = %d, want 0 (stderr: %s)", args, code, errOut.String())
		}
		if errOut.Len() != 0 {
			t.Fatalf("Run(%v) wrote to stderr, want usage on stdout only: %s", args, errOut.String())
		}
		text := out.String()
		for _, want := range []string{"cache", "dashboard", "prefix", "ttl", "fix", "version", "--days", "--json", "--no-tui", "--lang", "ATS_LANG", "NO_COLOR"} {
			if !strings.Contains(text, want) {
				t.Errorf("Run(%v) usage output missing %q:\n%s", args, want, text)
			}
		}
	}
}

// TestSubcommandHelpFlagPrintsUsageToStdoutAndExitsZero is the
// `ats <cmd> --help` counterpart: today it falls into the subcommand's
// own flag.FlagSet, which prints a one-line "Usage of cache:" to
// stderr and returns exit code 2.
func TestSubcommandHelpFlagPrintsUsageToStdoutAndExitsZero(t *testing.T) {
	for _, cmd := range []string{"cache", "dashboard", "prefix", "ttl", "fix"} {
		var out, errOut bytes.Buffer
		code := Run([]string{cmd, "--help"}, &out, &errOut, Env{})
		if code != 0 {
			t.Fatalf("Run([%s --help]) exit code = %d, want 0 (stderr: %s)", cmd, code, errOut.String())
		}
		if !strings.Contains(out.String(), "Commands:") && !strings.Contains(out.String(), "Commandes :") {
			t.Fatalf("Run([%s --help]) usage missing the commands list:\n%s", cmd, out.String())
		}
	}
}
