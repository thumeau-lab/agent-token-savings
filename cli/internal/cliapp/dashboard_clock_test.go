package cliapp

import (
	"bytes"
	"testing"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestDashboardGoldenModelClockIsFixedAnchor pins bug card point 1's
// root cause: the Dashboard golden fixture's "now" must be a fixed
// anchor (dashboardGoldenNow), never the real wall clock -- otherwise
// the goldens' day labels (Activite heatmap, Depenses quotidiennes,
// hero mini-stats) silently shift every time the real calendar day
// changes, and TestDashboardGolden* starts failing on whatever day
// someone happens to run `go test`.
func TestDashboardGoldenModelClockIsFixedAnchor(t *testing.T) {
	m := buildDashboardModel(t, 150, 45, i18n.EN, true)
	got := m.now()
	if !got.Equal(dashboardGoldenNow) {
		t.Fatalf("buildDashboardModel's m.now() = %v, want the fixed anchor %v (goldens must not depend on the real wall clock)", got, dashboardGoldenNow)
	}
}

// TestDashboardGoldenIsStableAcrossRealWallClock is the card's explicit
// "run goldens with two different fake real dates -> same output"
// check, without needing `faketime`: real wall-clock time genuinely
// elapses between the two buildDashboardModel calls below, so if
// anything in the golden's path still read time.Now() directly instead
// of the fixed anchor, this would be flaky across a date boundary.
func TestDashboardGoldenIsStableAcrossRealWallClock(t *testing.T) {
	m1 := buildDashboardModel(t, 150, 45, i18n.EN, true)
	out1 := m1.View().Content
	time.Sleep(5 * time.Millisecond)
	m2 := buildDashboardModel(t, 150, 45, i18n.EN, true)
	out2 := m2.View().Content
	if out1 != out2 {
		t.Fatalf("dashboard output changed between two calls separated by real wall-clock time, want identical:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
}

// TestDashboardCommandOutputStableUnderInjectedClock is the
// command-level (non-TUI `ats dashboard`) counterpart: with Env.Now
// pinned, the command's output must not depend on the real moment it
// runs. Two calls with the exact same injected clock, separated by
// real elapsed wall-clock time, must render byte-identical output --
// otherwise some code path (dashboard_cmd.go / cache.go's Copilot
// query) is still reading the real time.Now() behind Env.Now's back.
func TestDashboardCommandOutputStableUnderInjectedClock(t *testing.T) {
	home := t.TempDir()
	writeClaudeFixture(t, home)
	fixedClock := func() time.Time { return dashboardGoldenNow }

	run := func() string {
		var out, errOut bytes.Buffer
		code := Run([]string{"dashboard"}, &out, &errOut, Env{HomeDir: home, IsTTY: true, Lang: i18n.EN, Now: fixedClock})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
		}
		return out.String()
	}

	out1 := run()
	time.Sleep(5 * time.Millisecond)
	out2 := run()
	if out1 != out2 {
		t.Fatalf("ats dashboard output changed between two runs with the same injected clock, want identical:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
}
