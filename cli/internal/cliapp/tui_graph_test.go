package cliapp

import (
	"strings"
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestTUICachePanelShowsDailyHistoryGraphWhenEnoughDays verifies the
// Cache panel renders a per-day sparkline block (hit rate + API-equivalent
// cost) once at least 2 days of history are loaded, built from the exact
// per-day rows hermesusage.QueryDaily already returns (no second parser).
func TestTUICachePanelShowsDailyHistoryGraphWhenEnoughDays(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: false, Dark: true, Lang: i18n.EN}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{
		days:         7,
		cacheResults: []AgentCacheResult{demoCacheResult(), demoHermesCacheResult()},
		dailyHermes: []hermesusage.DailyRow{
			{Day: "2026-10-10", Sessions: 1, Calls: 10, Output: 100, Input: 50, CacheRead: 1000, CacheWrite: 200},
			{Day: "2026-10-11", Sessions: 1, Calls: 12, Output: 120, Input: 60, CacheRead: 500, CacheWrite: 900},
		},
	})
	m.detected = []agents.Kind{agents.ClaudeCode, agents.Hermes}
	m.agentIdx = 1 // hermes
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	content := stripANSIForTest(m.View().Content)
	if !strings.Contains(content, "Hit rate") {
		t.Fatalf("Cache panel does not show the per-day history graph with 2 days loaded:\n%s", content)
	}
}

// demoHermesCacheResult is the hermes counterpart to demoCacheResult,
// for tests exercising the agent selector/per-day graph on hermes.
func demoHermesCacheResult() AgentCacheResult {
	return AgentCacheResult{
		Agent: "hermes", Days: 7, Sessions: 2, Calls: 22,
		CacheWriteTokens: 1_100, CacheReadTokens: 1_500,
		InputTokens: 110, OutputTokens: 220, HitRate: 0.577,
	}
}

// stripANSIForTest removes ANSI CSI escapes, for assertions on visible text.
func stripANSIForTest(s string) string {
	var out strings.Builder
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:i])
		s = s[i+2:]
		j := strings.IndexAny(s, "mK")
		if j < 0 {
			break
		}
		s = s[j+1:]
	}
	return out.String()
}

// TestTUICachePanelGraphShowsMinMaxAndFirstLastDay is the regression
// test for the per-day history sparklines having no axis labels at all
// (reviewer feedback): the graph must show each series' min/max on the
// right, and the first/last day (short, localized) under the graph.
func TestTUICachePanelGraphShowsMinMaxAndFirstLastDay(t *testing.T) {
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: false, Dark: true, Lang: i18n.FR}, testRoots(home))
	m = resize(t, m, 100, 30)
	m = feedLoaded(t, m, loadedMsg{
		days:         7,
		cacheResults: []AgentCacheResult{demoCacheResult(), demoHermesCacheResult()},
		dailyHermes: []hermesusage.DailyRow{
			{Day: "2026-10-07", Sessions: 1, Calls: 10, Output: 100, Input: 50, CacheRead: 100, CacheWrite: 900},
			{Day: "2026-10-11", Sessions: 1, Calls: 12, Output: 120, Input: 60, CacheRead: 1000, CacheWrite: 200},
		},
	})
	m.detected = []agents.Kind{agents.ClaudeCode, agents.Hermes}
	m.agentIdx = 1 // hermes
	m.skipped = nil
	m.tab = tabCache
	m.refreshBody()

	content := stripANSIForTest(m.View().Content)
	if !strings.Contains(content, "min") || !strings.Contains(content, "max") {
		t.Fatalf("Cache panel graph does not show min/max labels:\n%s", content)
	}
	// 2026-10-07 is the oldest day, 2026-10-11 the most recent: French
	// short dates are day/month, so "07/10" and "11/10".
	if !strings.Contains(content, "07/10") || !strings.Contains(content, "11/10") {
		t.Fatalf("Cache panel graph does not show the first/last day (short, localized French date):\n%s", content)
	}
}
