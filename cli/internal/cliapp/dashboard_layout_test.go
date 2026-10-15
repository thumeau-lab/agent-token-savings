package cliapp

import (
	"strings"
	"testing"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// dashboardLayoutFixture is a small dashboardData with just enough in
// every section (hero, efficiency, heatmap, histogram, agent rows,
// model rows, findings, measured) to exercise every panel the new
// responsive layout wires in.
func dashboardLayoutFixture() dashboardData {
	cache := []AgentCacheResult{{Agent: "hermes", Calls: 100, HitRate: 0.9}}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	return buildDashboard(
		cache, nil, fixer.Finding{}, false,
		nil, nil, nil, now, 7,
		comparisonResult{}, comparisonResult{}, nil, "", i18n.EN,
	)
}

// TestRenderDashboardWideShowsEveryNewPanel pins card point 1's wide
// (>=150) layout: hero, Efficacite, Activite, Taux de cache par agent,
// Depenses quotidiennes, A ameliorer, Par modele and Mesure must all
// appear somewhere in the rendered output.
func TestRenderDashboardWideShowsEveryNewPanel(t *testing.T) {
	d := dashboardLayoutFixture()
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboard(d, r, 200, 50, i18n.EN, true)
	for _, want := range []string{
		"Efficiency", "Activity", "Cache rate by agent",
		"Daily spend", "To improve", "By model", "Measured",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderDashboard(200,50) missing panel %q:\n%s", want, out)
		}
	}
}

// TestDashboardGolden200x50HasNoEmptyRowsAboveFooter is the card's
// explicit point 3 regression test: at 200x50 with the full demo
// fixture (every panel populated, same as the committed goldens), the
// histogram must absorb every row dashboardHistogramRows computes as
// "remaining" instead of leaving it capped and padded with blank
// lines right above the footer (card: "raise the 12-row cap to fill
// exactly").
func TestDashboardGolden200x50HasNoEmptyRowsAboveFooter(t *testing.T) {
	m := buildDashboardModel(t, 200, 50, i18n.EN, true)
	content := m.View().Content
	lines := strings.Split(content, "\n")
	// The footer is always the very last line; count the blank lines
	// directly above it (the viewport pads with these when the
	// rendered dashboard is shorter than the body height).
	blankAboveFooter := 0
	for i := len(lines) - 2; i >= 0 && strings.TrimSpace(lines[i]) == ""; i-- {
		blankAboveFooter++
	}
	if blankAboveFooter > 1 {
		t.Fatalf("dashboard View() at 200x50 has %d blank lines right above the footer, want <= 1 (the histogram must fill the remaining rows exactly):\n%s", blankAboveFooter, content)
	}
}

// TestRenderDashboardWideHistogramAbsorbsRemainingRows is the card's
// explicit anti-regression test: "no empty bottom half at 200x50" --
// at a generous height the histogram panel must grow well past its
// floor instead of leaving blank trailing lines.
func TestRenderDashboardWideHistogramAbsorbsRemainingRows(t *testing.T) {
	d := dashboardLayoutFixture()
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboard(d, r, 200, 50, i18n.EN, true)
	lines := strings.Split(out, "\n")
	blankTrailing := 0
	for i := len(lines) - 1; i >= 0 && strings.TrimSpace(lines[i]) == ""; i-- {
		blankTrailing++
	}
	if blankTrailing > 2 {
		t.Fatalf("renderDashboard(200,50) has %d blank trailing lines, want <= 2 (no empty bottom half):\n%s", blankTrailing, out)
	}
}

// TestDashboardHistogramRowsHasAFloorOfFive pins dashboardHistogramRows'
// own contract directly: never below the card's "min 5", and it grows
// with more available height.
func TestDashboardHistogramRowsHasAFloorOfFive(t *testing.T) {
	if got := dashboardHistogramRows(10, 100); got != dashboardHistogramMinRows {
		t.Fatalf("dashboardHistogramRows(10, 100) = %d, want the floor %d", got, dashboardHistogramMinRows)
	}
	if got := dashboardHistogramRows(50, 20); got <= dashboardHistogramMinRows {
		t.Fatalf("dashboardHistogramRows(50, 20) = %d, want it to grow past the floor with room to spare", got)
	}
	if got := dashboardHistogramRows(0, 20); got != dashboardHistogramDefaultRows {
		t.Fatalf("dashboardHistogramRows(0, 20) = %d, want the fixed default %d when no height is known", got, dashboardHistogramDefaultRows)
	}
}

// TestDashboardHistogramRowsHasAnUpperCap pins the raised ceiling (card
// t_9194698a point 3: "raise the 12-row cap to fill exactly" -- the old
// cap=12 left ~15 computed remaining rows at 200x50 underused, visible
// as blank lines above the footer): a pathologically tall terminal
// must still not let the chart grow without bound.
func TestDashboardHistogramRowsHasAnUpperCap(t *testing.T) {
	if got := dashboardHistogramRows(2000, 20); got != dashboardHistogramMaxRows {
		t.Fatalf("dashboardHistogramRows(2000, 20) = %d, want the cap %d", got, dashboardHistogramMaxRows)
	}
}

// TestDashboardHistogramRowsCapIsTwelve is the card point 4 regression
// test: at 7 j the bar chart used to grow tall enough to "eat half the
// screen" (the previous card had raised the cap to 40 to fill a 200x50
// terminal exactly). The cap must come back down to a sane ceiling so
// a handful of days never draws giant bars; the freed rows go to the
// Activite/Taux par agent panels instead (see
// TestRenderDashboardWideGivesFreedHistogramRowsToRow2PanelsWithoutWastingSpace).
func TestDashboardHistogramRowsCapIsTwelve(t *testing.T) {
	if got := dashboardHistogramRows(2000, 20); got != 12 {
		t.Fatalf("dashboardHistogramRows(2000, 20) = %d, want 12", got)
	}
}

// TestRenderDashboardWideGivesFreedHistogramRowsToRow2PanelsWithoutWastingSpace
// is the card point 4 regression test for the lowered histogram cap:
// lowering dashboardHistogramMaxRows alone would leave blank rows
// above the footer whenever the real "remaining" height exceeds the
// new cap. Those freed rows must go to the Activite/Taux par agent
// panels instead (card: "give the freed rows to Activite / Taux par
// agent panels"), so the wide layout's total rendered height still
// matches the real terminal height exactly, no wasted space.
func TestRenderDashboardWideGivesFreedHistogramRowsToRow2PanelsWithoutWastingSpace(t *testing.T) {
	d := dashboardLayoutFixture()
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	const height = 50
	out := renderDashboard(d, r, 200, height, i18n.EN, true)
	lines := strings.Split(out, "\n")
	if len(lines) != height {
		t.Fatalf("renderDashboard(200,%d) produced %d lines, want exactly %d (freed histogram rows must go to row2 panels, not be wasted)", height, len(lines), height)
	}
}

// TestDashboardActivityBoxWidthShrinksToContentNotHalfColumn is the
// card point 5 regression test: the Activite heatmap only ever needs
// heatmapGutterWidth + dashboardHeatmapWeeks*heatmapCellWidth columns
// of content (a fixed 12-week grid), so giving it a full half-width
// column at 200 cols wasted ~3/4 of its own panel width. The box must
// shrink to its content width, handing the recovered columns to its
// row2 neighbor (Taux de cache par agent) instead of sitting mostly
// empty.
func TestDashboardActivityBoxWidthShrinksToContentNotHalfColumn(t *testing.T) {
	half := 99 // (200-1)/2, the wide layout's column width at 200 cols
	got := dashboardActivityBoxWidth(half)
	if got >= half {
		t.Fatalf("dashboardActivityBoxWidth(half=%d) = %d, want it shrunk well below half (the heatmap was using ~1/4 of its panel width)", half, got)
	}
	want := heatmapGutterWidth + dashboardHeatmapWeeks*heatmapCellWidth + 4
	if got != want {
		t.Fatalf("dashboardActivityBoxWidth(half=%d) = %d, want %d (gutter + weeks*cellWidth + box borders/padding)", half, got, want)
	}
}

// TestDashboardActivityBoxWidthNeverExceedsItsFairShare: on a narrower
// terminal where even half a column is less than the heatmap's own
// content width, the box must never grow past what row2 actually has
// to give it.
func TestDashboardActivityBoxWidthNeverExceedsItsFairShare(t *testing.T) {
	half := 20
	if got := dashboardActivityBoxWidth(half); got != half {
		t.Fatalf("dashboardActivityBoxWidth(half=%d) = %d, want capped at half", half, got)
	}
}

// TestRenderPanelHistogramSpreadsBarsAcrossFullWidth pins card point
// 4's main fix: with few days and a wide panel, bars must occupy most
// of the available width (bar width = (panelWidth-axis)/days), not a
// handful of columns stuck at the far left.
func TestRenderPanelHistogramSpreadsBarsAcrossFullWidth(t *testing.T) {
	d := dashboardData{Histogram: dashboardHistogram{
		Bars: []histogramBar{
			{Day: "2026-10-08", Cost: 1}, {Day: "2026-10-09", Cost: 3},
			{Day: "2026-10-10", Cost: 5}, {Day: "2026-10-11", Cost: 2},
			{Day: "2026-10-12", Cost: 8}, {Day: "2026-10-13", Cost: 4},
			{Day: "2026-10-14", Cost: 12},
		},
		AvgPerDay: 5, Peak: 12, PeakDay: "2026-10-14", Yesterday: 4, YesterdayKnown: true,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelHistogram(d, r, i18n.EN, 8, 190)
	maxBarRunLength := 0
	for _, line := range lines {
		run := 0
		for _, ch := range line {
			if ch == '\u2588' || ch == '.' {
				run++
				if run > maxBarRunLength {
					maxBarRunLength = run
				}
			} else {
				run = 0
			}
		}
	}
	// 7 days spread over a ~180-col bars area must give each bar a
	// double-digit width, nowhere close to the old ~1-col-per-day bug.
	if maxBarRunLength < 15 {
		t.Fatalf("renderPanelHistogram(width=190) longest bar run = %d cols, want bars spread wide (>=15): %v", maxBarRunLength, lines)
	}
}

// TestRenderPanelHistogramShowsAxisAndDateLabels pins the rest of
// card point 4: a $ axis (0, mid, max) on the right, and date labels
// under the first/middle/last bar.
func TestRenderPanelHistogramShowsAxisAndDateLabels(t *testing.T) {
	d := dashboardData{Histogram: dashboardHistogram{
		Bars: []histogramBar{
			{Day: "2026-10-08", Cost: 1}, {Day: "2026-10-09", Cost: 3},
			{Day: "2026-10-10", Cost: 5}, {Day: "2026-10-11", Cost: 2},
			{Day: "2026-10-12", Cost: 8}, {Day: "2026-10-13", Cost: 4},
			{Day: "2026-10-14", Cost: 12},
		},
		AvgPerDay: 5, Peak: 12, PeakDay: "2026-10-14", Yesterday: 4, YesterdayKnown: true,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelHistogram(d, r, i18n.EN, 8, 190)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"$0", "$12", "10/08", "10/14"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("renderPanelHistogram() missing %q:\n%s", want, joined)
		}
	}
}

// TestRenderDashboardMediumHidesActivityBelow120 pins "Activite hidden
// below 120" at the 100-149 tier.
func TestRenderDashboardMediumHidesActivityBelow120(t *testing.T) {
	d := dashboardLayoutFixture()
	r := render.New(render.Options{IsTTY: true, NoColor: true})

	below := renderDashboard(d, r, 110, 30, i18n.EN, true)
	if strings.Contains(below, "Activity") {
		t.Errorf("renderDashboard(110,30) shows Activity, want it hidden below 120 cols:\n%s", below)
	}

	above := renderDashboard(d, r, 130, 30, i18n.EN, true)
	if !strings.Contains(above, "Activity") {
		t.Errorf("renderDashboard(130,30) missing Activity, want it shown at >=120 cols:\n%s", above)
	}
}

// TestRenderDashboardNarrowIsSingleColumn pins "<100: one column": at
// 80 cols every panel box must span the full width (no two boxes
// joined side by side on the same line).
func TestRenderDashboardNarrowIsSingleColumn(t *testing.T) {
	d := dashboardLayoutFixture()
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboard(d, r, 80, 30, i18n.EN, false)
	for _, line := range strings.Split(out, "\n") {
		if w := len([]rune(line)); w > 80 {
			t.Fatalf("renderDashboard(80,30) line %q is %d cols wide, want <= 80 (single column)", line, w)
		}
	}
}
