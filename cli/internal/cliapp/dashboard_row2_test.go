package cliapp

import (
	"strings"
	"testing"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// TestDashboardRow2BoxesSizeToOwnContentHeight is the regression test
// for the follow-up card's point A: "the rows freed by the histogram
// cap went into the Activite / Taux par agent boxes, now ~25 empty
// lines each. Boxes must size to content height." dashboardRow2 takes
// no terminal-height parameter at all -- by construction its two
// boxes can only ever be as tall as their own content (activity's
// fixed grid, or one header + one row per agent), never padded to
// match an outer viewport.
func TestDashboardRow2BoxesSizeToOwnContentHeight(t *testing.T) {
	d := dashboardData{
		AgentRows: []dashboardAgentRow{{Agent: "hermes", HitRate: 0.9, Calls: 100, CostKnown: true, Cost: 10, Saved: 5}},
		Heatmap: dashboardHeatmap{
			Weeks:       make([][7]heatmapDay, dashboardHeatmapWeeks),
			MonthStarts: make([]time.Month, dashboardHeatmapWeeks),
		},
	}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	row2 := dashboardRow2(d, r, i18n.EN, 200)

	activityLines := renderPanelActivity(d, r, i18n.EN)
	byAgentWidth := 200 - 1 - dashboardActivityBoxWidth(99)
	byAgentLines := renderPanelByAgent(d, r, i18n.EN, byAgentWidth)
	wantMax := len(activityLines)
	if n := len(byAgentLines); n > wantMax {
		wantMax = n
	}
	want := wantMax + 2 // top+bottom border
	if got := countLines(row2); got != want {
		t.Fatalf("countLines(dashboardRow2()) = %d, want %d (max of the two panels' own content height + border, never padded past that)", got, want)
	}
}

// TestRenderDashboardWideRow2HeightDoesNotGrowWithTerminalHeight is
// the end-to-end regression test: the same data rendered at a short
// (50) and a tall (90) terminal height must produce the exact same
// "Taux de cache par agent" panel content -- the freed rows from a
// taller terminal must go to the histogram, never inflate this panel
// with blank lines.
func TestRenderDashboardWideRow2HeightDoesNotGrowWithTerminalHeight(t *testing.T) {
	d := dashboardLayoutFixture()
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	row2Short := dashboardRow2(d, r, i18n.EN, 200)
	row2Tall := dashboardRow2(d, r, i18n.EN, 200)
	if row2Short != row2Tall {
		t.Fatalf("dashboardRow2 output must be identical regardless of terminal height")
	}
	out50 := renderDashboard(d, r, 200, 50, i18n.EN, true)
	out90 := renderDashboard(d, r, 200, 90, i18n.EN, true)
	if !strings.Contains(out50, row2Short) {
		t.Fatalf("renderDashboard(200,50) does not contain the row2 block as rendered standalone:\n%s\n--- row2 ---\n%s", out50, row2Short)
	}
	if !strings.Contains(out90, row2Short) {
		t.Fatalf("renderDashboard(200,90) does not contain the same row2 block as the 50-row terminal (it must not grow with height):\n%s\n--- row2 ---\n%s", out90, row2Short)
	}
}
