package cliapp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// TestTTYColorProfileDetectsTmux256Color pins the card's "color in
// tmux" fix: a TTY env whose real process environment advertises
// TERM=tmux-256color must resolve to at least ANSI256, not NoTTY --
// the bug this card fixes is runTUI never passing an explicit profile
// to tea.NewProgram, leaving it to bubbletea's own isatty check on
// p.output (which can disagree with the IsTTY this process already
// established once in main.go, e.g. under tmux/ssh pty forwarding).
func TestTTYColorProfileDetectsTmux256Color(t *testing.T) {
	got := ttyColorProfile(Env{IsTTY: true}, []string{"TERM=tmux-256color"})
	if got < colorprofile.ANSI256 {
		t.Fatalf("ttyColorProfile(TERM=tmux-256color) = %v, want >= ANSI256", got)
	}
}

// TestTTYColorProfileNoColorStripsAll: NO_COLOR must win over any
// terminal capability, down to NoTTY (ansi.Strip discards every escape
// sequence, not just colors -- card: "NO_COLOR=1 -> none").
func TestTTYColorProfileNoColorStripsAll(t *testing.T) {
	got := ttyColorProfile(Env{IsTTY: true, NoColor: true}, []string{"TERM=tmux-256color"})
	if got != colorprofile.NoTTY {
		t.Fatalf("ttyColorProfile(NoColor=true) = %v, want NoTTY", got)
	}
}

// TestTTYColorProfileNotATTYStripsAll: a non-interactive output (piped,
// redirected) must never emit color regardless of TERM.
func TestTTYColorProfileNotATTYStripsAll(t *testing.T) {
	got := ttyColorProfile(Env{IsTTY: false}, []string{"TERM=tmux-256color"})
	if got != colorprofile.NoTTY {
		t.Fatalf("ttyColorProfile(IsTTY=false) = %v, want NoTTY", got)
	}
}

// TestDashboardSGRSurvivesForcedColorProfile is the card's literal
// test: a Colored() dashboard View (IsTTY, NoColor unset) still shows
// SGR color codes once downsampled through the forced tmux-256color
// profile -- proving the wiring keeps color alive instead of letting
// bubbletea's own (possibly disagreeing) isatty probe silently strip
// it, as it would if the profile stayed NoTTY.
func TestDashboardSGRSurvivesForcedColorProfile(t *testing.T) {
	home := t.TempDir()
	env := Env{HomeDir: home, IsTTY: true, NoColor: false, Dark: true}
	m := newTUIModel(env, testRoots(home))
	m = resize(t, m, 100, 30)
	view := m.View().Content

	profile := ttyColorProfile(env, []string{"TERM=tmux-256color"})
	var buf bytes.Buffer
	w := &colorprofile.Writer{Forward: &buf, Profile: profile}
	if _, err := w.WriteString(view); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected SGR escape codes under a forced 256-color profile, got %q", buf.String())
	}
}

// TestDashboardSGRNoneUnderNoColor: with NO_COLOR semantics (env.NoColor
// true), render.Renderer.Colored() is already false so the View never
// embeds a single ANSI escape to begin with -- downsampling through any
// profile must still show none.
func TestDashboardSGRNoneUnderNoColor(t *testing.T) {
	home := t.TempDir()
	env := Env{HomeDir: home, IsTTY: true, NoColor: true}
	m := newTUIModel(env, testRoots(home))
	m = resize(t, m, 100, 30)
	view := m.View().Content

	profile := ttyColorProfile(env, []string{"TERM=tmux-256color"})
	var buf bytes.Buffer
	w := &colorprofile.Writer{Forward: &buf, Profile: profile}
	if _, err := w.WriteString(view); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected no SGR escape codes under NO_COLOR, got %q", buf.String())
	}
}
