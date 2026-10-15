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

// TestRenderPanelByModelGroupsRowsByAgent is the regression test for
// the "same model name listed twice with no agent marker" bug: when
// several agents are in scope (card: "agent 'tous'"), each agent gets
// its own dim sub-header line and its models are listed under it, so
// e.g. "claude-opus-5" reported by both claude-code and hermes reads
// as two distinct rows under two distinct headers, never one
// unreadable pile. Point B of the follow-up card: this must hold for
// every agent, not just claude-code/hermes -- codex and copilot too.
func TestRenderPanelByModelGroupsRowsByAgent(t *testing.T) {
	d := dashboardData{ModelRows: []dashboardModelRow{
		{Agent: "claude-code", Model: "claude-opus-5", HitRate: 0.7, Calls: 40},
		{Agent: "hermes", Model: "claude-opus-5", HitRate: 0.6, Calls: 20},
		{Agent: "codex", Model: "gpt-5.6-luna", HitRate: 0.5, Calls: 10},
		{Agent: "copilot", Model: "gpt-5.6-luna", HitRate: 0.4, Calls: 5},
	}}
	r := render.New(render.Options{NoColor: true})
	lines := renderPanelByModel(d, r, i18n.EN, 80)

	for _, agent := range []string{"claude-code", "hermes", "codex", "copilot"} {
		found := false
		for _, l := range lines {
			if strings.TrimSpace(l) == agent {
				found = true
			}
		}
		if !found {
			t.Fatalf("renderPanelByModel() = %v, missing a dim sub-header line for agent %q", lines, agent)
		}
	}
}

// TestRenderPanelByModelSortsModelsByCallsDescWithinAgent pins the
// card's explicit sort rule: inside one agent's group, models are
// ordered busiest-first.
func TestRenderPanelByModelSortsModelsByCallsDescWithinAgent(t *testing.T) {
	d := dashboardData{ModelRows: []dashboardModelRow{
		{Agent: "hermes", Model: "small-model", HitRate: 0.5, Calls: 5},
		{Agent: "hermes", Model: "big-model", HitRate: 0.5, Calls: 500},
	}}
	r := render.New(render.Options{NoColor: true})
	lines := renderPanelByModel(d, r, i18n.EN, 80)
	joined := strings.Join(lines, "\n")
	if strings.Index(joined, "big-model") > strings.Index(joined, "small-model") {
		t.Fatalf("renderPanelByModel() = %q, want big-model (more calls) listed before small-model", joined)
	}
}

// TestRenderPanelByModelOmitsHeaderWhenScopedToOneAgent pins "when
// the agent selector is one agent, show only its models": with a
// single agent in ModelRows (buildDashboard already filters to the
// selected agent, see the scopeAgent loop), there is nothing to
// disambiguate, so no redundant agent sub-header should appear.
func TestRenderPanelByModelOmitsHeaderWhenScopedToOneAgent(t *testing.T) {
	d := dashboardData{ModelRows: []dashboardModelRow{
		{Agent: "hermes", Model: "claude-opus-5", HitRate: 0.7, Calls: 40},
	}}
	r := render.New(render.Options{NoColor: true})
	lines := renderPanelByModel(d, r, i18n.EN, 80)
	for _, l := range lines {
		if strings.TrimSpace(l) == "hermes" {
			t.Fatalf("renderPanelByModel() = %v, want no agent sub-header when only one agent is in scope", lines)
		}
	}
}
