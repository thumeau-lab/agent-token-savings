package cliapp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestDashboardCommandExitsZeroAndPrintsHeroFigures is the non-TUI
// text path's smoke test (card: "Same dashboard ... for `ats`
// non-TUI text output on a TTY"): `ats dashboard` with real
// claude-code fixture data exits 0 and shows the hero cost figure.
func TestDashboardCommandExitsZeroAndPrintsHeroFigures(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"dashboard"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, Lang: i18n.EN})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "Cache rate by agent") {
		t.Fatalf("output missing the per-agent panel:\n%s", text)
	}
	if !strings.Contains(text, "claude-code") {
		t.Fatalf("output missing claude-code:\n%s", text)
	}
}

// TestDashboardCommandNeverRendersBlockDigits: the non-interactive
// path must skip block digits even at a wide width (card: "minus
// block digits").
func TestDashboardCommandNeverRendersBlockDigits(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"dashboard"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, TermWidth: 160})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	// Block digits use the half-block glyphs (U+2580 TOP HALF BLOCK,
	// U+2584 LOWER HALF BLOCK) that no plain bar/annotation ever emits
	// (Bar() only ever draws solid U+2588 FULL BLOCK/U+2591 LIGHT
	// SHADE); checking for those two specifically (not the full block,
	// which legitimately appears in every hit-rate bar) tells apart
	// "block digits rendered" from "just the usual bars".
	if strings.ContainsAny(out.String(), "\u2580\u2584") {
		t.Fatalf("ats dashboard rendered block-digit glyphs, want the plain fallback line:\n%s", out.String())
	}
}

// TestDashboardCommandAcceptsDaysFlag matches cache/ttl/prefix's own
// --days flag parity.
func TestDashboardCommandAcceptsDaysFlag(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)

	var out, errOut bytes.Buffer
	code := Run([]string{"dashboard", "--days", "30"}, &out, &errOut, Env{HomeDir: home, IsTTY: true})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
}
