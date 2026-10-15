package cliapp

import (
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestRenderHeaderShowsAllOnDashboardWhenScopeIsCombined is the card
// point 3 regression test: pressing "p" alone (SwitchAgent) only
// cycles which agent "a" (ToggleScope) would later scope the
// Dashboard to -- it does not itself filter the Dashboard. Showing
// the cycled agent's name in the header while the Dashboard still
// aggregates every agent was misleading ("agent : < hermes > (2/2)"
// with nothing on the dashboard actually filtered). The header must
// say "tous"/"all" instead whenever allAgents is still true.
func TestRenderHeaderShowsAllOnDashboardWhenScopeIsCombined(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.FR}, testRoots(home))
	m.detected = []agents.Kind{agents.Hermes, agents.ClaudeCode}
	m.agentIdx = 1 // "p" cycled to claude-code
	m.allAgents = true
	m.tab = tabDashboard

	header := m.renderHeader()
	lines := strings.Split(header, "\n")
	secondLine := lines[1]
	if strings.Contains(secondLine, "claude-code") {
		t.Fatalf("renderHeader() second line = %q, dashboard is not scoped (allAgents=true) so it must not show the cycled agent's name", secondLine)
	}
	if !strings.Contains(secondLine, "tous") {
		t.Fatalf("renderHeader() second line = %q, want it to say \"tous\" (all) while the dashboard scope is combined", secondLine)
	}
}

// TestRenderHeaderShowsSpecificAgentOnDashboardWhenScoped: once "a"
// has actually scoped the dashboard (allAgents=false), the header must
// show the specific selected agent, not "tous".
func TestRenderHeaderShowsSpecificAgentOnDashboardWhenScoped(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.FR}, testRoots(home))
	m.detected = []agents.Kind{agents.Hermes, agents.ClaudeCode}
	m.agentIdx = 1
	m.allAgents = false
	m.tab = tabDashboard

	header := m.renderHeader()
	if !strings.Contains(header, "claude-code") {
		t.Fatalf("renderHeader() = %q, dashboard is scoped to claude-code, want its name shown", header)
	}
	if strings.Contains(header, "tous") {
		t.Fatalf("renderHeader() = %q, must not say \"tous\" once the dashboard is scoped to one agent", header)
	}
}

// TestRenderHeaderShowsSpecificAgentOnOtherTabsRegardlessOfScope: on
// every non-Dashboard tab, "p" already fully controls which agent's
// card is shown (no separate all/one toggle there), so the header
// must keep showing the cycled agent's name regardless of allAgents.
func TestRenderHeaderShowsSpecificAgentOnOtherTabsRegardlessOfScope(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: true, Lang: i18n.FR}, testRoots(home))
	m.detected = []agents.Kind{agents.Hermes, agents.ClaudeCode}
	m.agentIdx = 1
	m.allAgents = true
	m.tab = tabCache

	header := m.renderHeader()
	if !strings.Contains(header, "claude-code") {
		t.Fatalf("renderHeader() = %q, the Cache tab must keep showing the cycled agent regardless of the dashboard's own scope flag", header)
	}
}
