package cliapp

import (
	"strings"
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// dashboardDemoLoadedMsg builds a rich fixture exercising every
// dashboard panel at once: two agents with known pricing (claude-code,
// hermes -- one with an abnormal warm rebuild), one with unknown
// pricing and a per-model breakdown (copilot, two models -> triggers
// the model-switch finding), a worthwhile TTL saving (hermes), a
// pending Hermes config fix, and per-day history for both claude-code
// and hermes so panel C has real rows.
func dashboardDemoLoadedMsg() loadedMsg {
	claude := demoCacheResult()
	claude.USD, claude.USDShare = usdSplit(claude.InputTokens, claude.OutputTokens, claude.CacheReadTokens, claude.CacheWriteTokens)
	claude.WarmRebuilds = 1
	claude.WarmRebuildDetails = []WarmRebuildDetail{{Session: "sess-1", Timestamp: "2026-10-10T10:00:00Z", Tokens: 80_000}}

	hermes := AgentCacheResult{
		Agent: "hermes", Days: 7, Sessions: 40, Calls: 900,
		CacheWriteTokens: 2_000_000, CacheReadTokens: 30_000_000,
		InputTokens: 400_000, OutputTokens: 90_000, HitRate: 0.94,
	}
	hermes.USD, hermes.USDShare = usdSplit(hermes.InputTokens, hermes.OutputTokens, hermes.CacheReadTokens, hermes.CacheWriteTokens)

	copilot := AgentCacheResult{
		Agent: "copilot", Days: 7, Sessions: 5, Calls: 60,
		CacheWriteTokens: 300_000, CacheReadTokens: 1_000_000,
		InputTokens: 200_000, OutputTokens: 50_000, HitRate: 0.6,
		CostUnknown: true,
		// Model names long enough (24+ chars) to exercise card point
		// 2: at 200x50 the "Par modele" panel has ~60 free columns,
		// so these must render in full, not truncated to the old
		// hardcoded 12-column name column.
		ModelBreakdown: []ModelUsageJSON{
			{Model: "claude-opus-4-5-20251101", Calls: 40, HitRate: 0.7},
			{Model: "claude-sonnet-4-5-20250929", Calls: 20, HitRate: 0.3},
		},
	}

	codex := AgentCacheResult{
		Agent: "codex", Days: 7, Sessions: 8, Calls: 50,
		CacheReadTokens: 500_000, InputTokens: 100_000, OutputTokens: 20_000,
		HitRate: 0.5, CostUnknown: true, CacheWriteUnknown: true, SessionsWithoutUsage: 2,
	}

	ttlFinding := fixer.Finding{
		Agent: "hermes", Key: fixer.HermesCacheTTLKey, OldValue: "1h", NewValue: "auto",
		Description: "prompt_caching.cache_ttl is \"1h\"",
	}

	dailyClaude := []claudecode.DailyRow{
		{Day: "2026-10-12", Create: 1_000_000, Read: 6_000_000, Input: 100_000, Output: 20_000},
		{Day: "2026-10-13", Create: 900_000, Read: 7_000_000, Input: 110_000, Output: 22_000},
	}
	dailyHermes := []hermesusage.DailyRow{
		{Day: "2026-10-12", Sessions: 5, Calls: 100, CacheRead: 3_000_000, CacheWrite: 200_000, Input: 40_000, Output: 9_000},
		{Day: "2026-10-13", Sessions: 6, Calls: 120, CacheRead: 3_500_000, CacheWrite: 180_000, Input: 45_000, Output: 10_000},
	}

	return loadedMsg{
		days:         7,
		cacheResults: []AgentCacheResult{claude, hermes, copilot, codex},
		ttlResults: []AgentTTLResult{
			{Agent: "hermes", Days: 7, EstimatedSaving: 0.3, TotalWriteTokens: 2_000_000},
		},
		fixFinding: ttlFinding, fixHasFinding: true,
		dailyClaude: dailyClaude, dailyHermes: dailyHermes,
		// loadedAt: production loadCmd always stamps this with the
		// same `now` it used to build everything else (see
		// tui.go:loadCmd); a fixture that leaves it zero makes
		// renderHeader() treat the Dashboard tab as "not loaded yet"
		// and show its own "skipped: ..." note on top of the
		// Dashboard's own Mesure panel repeating the same note (card
		// point 8: header dedup).
		loadedAt: dashboardGoldenNow,
	}
}

// dashboardDemoSkipped is the skipped-agent fixture for panel F's
// ignored-agents dim note (card point H).
func dashboardDemoSkipped(lang i18n.Lang) []skippedAgent {
	return []skippedAgent{{Kind: agents.GeminiCLI, Reason: i18n.T(lang, "skip.gemini")}}
}

// dashboardGoldenNow is the fixed clock anchor every Dashboard golden
// test builds against (card point 1: "inject a fixed now into every
// dashboard builder/loader used by goldens"). A Wednesday, matching
// TestBuildActivityHeatmapWeekAlignmentFixedClock's own fixture clock,
// one day after the fixture's last daily row (2026-10-13) so the hero
// mini-stats' "yesterday" lines up with real fixture data. Golden
// output must never depend on the real wall clock the test happens to
// run on -- see TestDashboardGoldenModelClockIsFixedAnchor and
// TestDashboardGoldenIsStableAcrossRealWallClock.
var dashboardGoldenNow = time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)

// buildDashboardModel builds a fully-loaded tuiModel on the Dashboard
// tab for the golden tests below: same fixture, varying only
// width/height/lang/color.
func buildDashboardModel(t *testing.T, width, height int, lang i18n.Lang, noColor bool) tuiModel {
	t.Helper()
	home := t.TempDir()
	m := newTUIModel(Env{HomeDir: home, IsTTY: true, NoColor: noColor, Dark: true, Lang: lang}, testRoots(home))
	m.now = func() time.Time { return dashboardGoldenNow }
	m = resize(t, m, width, height)
	m = feedLoaded(t, m, dashboardDemoLoadedMsg())
	m.detected = []agents.Kind{agents.ClaudeCode, agents.Hermes, agents.Copilot, agents.Codex}
	m.skipped = dashboardDemoSkipped(lang)
	m.tab = tabDashboard
	m.refreshBody()
	return m
}

func TestDashboardGolden100x30EN(t *testing.T) {
	m := buildDashboardModel(t, 100, 30, i18n.EN, true)
	checkTUIGolden(t, "dashboard_100x30_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGolden100x30FR(t *testing.T) {
	m := buildDashboardModel(t, 100, 30, i18n.FR, true)
	checkTUIGolden(t, "dashboard_100x30_fr.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGolden150x45EN(t *testing.T) {
	m := buildDashboardModel(t, 150, 45, i18n.EN, true)
	checkTUIGolden(t, "dashboard_150x45_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGolden150x45FR(t *testing.T) {
	m := buildDashboardModel(t, 150, 45, i18n.FR, true)
	checkTUIGolden(t, "dashboard_150x45_fr.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGolden200x50EN(t *testing.T) {
	m := buildDashboardModel(t, 200, 50, i18n.EN, true)
	checkTUIGolden(t, "dashboard_200x50_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGolden200x50FR(t *testing.T) {
	m := buildDashboardModel(t, 200, 50, i18n.FR, true)
	checkTUIGolden(t, "dashboard_200x50_fr.golden", stripTrailingSpaces(m.View().Content))
}

// TestDashboardGolden200x60EN / FR pin the follow-up card's new
// real-screen evidence size (200x60, 7 j): the "Taux de cache par
// agent" / "Activite" boxes must size to their own content (never ~25
// blank lines), with the freed rows going to the "Depenses
// quotidiennes" histogram instead -- see
// TestRenderDashboardWideFillsExactlyAtVariousHeights for the
// line-count regression test this golden complements.
func TestDashboardGolden200x60EN(t *testing.T) {
	m := buildDashboardModel(t, 200, 60, i18n.EN, true)
	checkTUIGolden(t, "dashboard_200x60_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGolden200x60FR(t *testing.T) {
	m := buildDashboardModel(t, 200, 60, i18n.FR, true)
	checkTUIGolden(t, "dashboard_200x60_fr.golden", stripTrailingSpaces(m.View().Content))
}

// TestDashboardGoldenColor100x30EN / FR / 150x45 / 200x50 pin the
// colored (non-NO_COLOR) run at each required size/language pair --
// the card's "color and NO_COLOR" matrix.
func TestDashboardGoldenColor100x30EN(t *testing.T) {
	m := buildDashboardModel(t, 100, 30, i18n.EN, false)
	checkTUIGolden(t, "dashboard_color_100x30_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGoldenColor100x30FR(t *testing.T) {
	m := buildDashboardModel(t, 100, 30, i18n.FR, false)
	checkTUIGolden(t, "dashboard_color_100x30_fr.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGoldenColor150x45EN(t *testing.T) {
	m := buildDashboardModel(t, 150, 45, i18n.EN, false)
	checkTUIGolden(t, "dashboard_color_150x45_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGoldenColor150x45FR(t *testing.T) {
	m := buildDashboardModel(t, 150, 45, i18n.FR, false)
	checkTUIGolden(t, "dashboard_color_150x45_fr.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGoldenColor200x50EN(t *testing.T) {
	m := buildDashboardModel(t, 200, 50, i18n.EN, false)
	checkTUIGolden(t, "dashboard_color_200x50_en.golden", stripTrailingSpaces(m.View().Content))
}

func TestDashboardGoldenColor200x50FR(t *testing.T) {
	m := buildDashboardModel(t, 200, 50, i18n.FR, false)
	checkTUIGolden(t, "dashboard_color_200x50_fr.golden", stripTrailingSpaces(m.View().Content))
}

// dashboardGoldenSizes pairs every dashboard golden file with the
// terminal width it was pinned at, for TestDashboardGoldenLinesFitWidth.
var dashboardGoldenSizes = map[string]int{
	"dashboard_100x30_en.golden":       100,
	"dashboard_100x30_fr.golden":       100,
	"dashboard_150x45_en.golden":       150,
	"dashboard_150x45_fr.golden":       150,
	"dashboard_200x50_en.golden":       200,
	"dashboard_200x50_fr.golden":       200,
	"dashboard_200x60_en.golden":       200,
	"dashboard_200x60_fr.golden":       200,
	"dashboard_color_100x30_en.golden": 100,
	"dashboard_color_100x30_fr.golden": 100,
	"dashboard_color_150x45_en.golden": 150,
	"dashboard_color_150x45_fr.golden": 150,
	"dashboard_color_200x50_en.golden": 200,
	"dashboard_color_200x50_fr.golden": 200,
}

// TestDashboardGoldenLinesFitWidth is the card's explicit test: "every
// golden line width <= terminal width (no wrap, borders intact)".
// lipgloss.Width measures visible columns, so ANSI escapes in the
// colored goldens do not inflate the count.
func TestDashboardGoldenLinesFitWidth(t *testing.T) {
	for name, width := range dashboardGoldenSizes {
		data, err := readGoldenFile(t, name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(data, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("%s line %d is %d columns wide, want <= %d:\n%q", name, i, w, width, line)
			}
		}
	}
}
