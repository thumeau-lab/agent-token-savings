package cliapp

import (
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// TestRenderPanelByModelShowsFullNameWhenPanelHasRoom is the
// regression test for the "Par modele" panel truncating model names
// ("claude-opus...", "claude-sonn...") even when the panel has ~60
// free columns at 200x50: the name column must be sized from the
// panel's own width (like renderPanelByAgent/renderPanelImprove
// already do), truncating only when the panel genuinely lacks room.
func TestRenderPanelByModelShowsFullNameWhenPanelHasRoom(t *testing.T) {
	longName := "claude-opus-4-5-20251101-preview"
	d := dashboardData{ModelRows: []dashboardModelRow{
		{Model: longName, HitRate: 0.7, Calls: 40},
	}}
	r := render.New(render.Options{NoColor: true})

	lines := renderPanelByModel(d, r, i18n.EN, 80)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, longName) {
		t.Fatalf("renderPanelByModel(width=80) = %q, want the full model name %q (panel has plenty of room)", joined, longName)
	}
}

// TestRenderPanelByModelTruncatesWhenPanelIsNarrow pins the other
// side: a genuinely narrow panel must still truncate (never overflow
// the box), same contract as renderPanelByAgent's agent-name column.
func TestRenderPanelByModelTruncatesWhenPanelIsNarrow(t *testing.T) {
	longName := "claude-opus-4-5-20251101-preview"
	d := dashboardData{ModelRows: []dashboardModelRow{
		{Model: longName, HitRate: 0.7, Calls: 40},
	}}
	r := render.New(render.Options{NoColor: true})

	lines := renderPanelByModel(d, r, i18n.EN, 30)
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, longName) {
		t.Fatalf("renderPanelByModel(width=30) = %q, want the long name truncated (panel is narrow)", joined)
	}
}
