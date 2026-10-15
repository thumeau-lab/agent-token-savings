package cliapp

import (
	"testing"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// TestBuildFindingsCopilotSwitchOnlyForCopilotAgent is the regression
// test for the false positive introduced once claude-code also gained
// a ModelBreakdown (7e3d4d1): the "model switch empties the cache"
// finding is a Copilot-specific anomaly (Copilot auto-routes between
// models mid-session, invalidating the prompt cache every switch;
// claude-code's per-model breakdown is just a usage-by-model report,
// no auto-routing, no equivalent anomaly) and must never fire for any
// other agent just because it happens to report more than one model.
func TestBuildFindingsCopilotSwitchOnlyForCopilotAgent(t *testing.T) {
	copilotLabel := i18n.T(i18n.FR, "dashboard.finding_copilot_switch")

	claude := AgentCacheResult{
		Agent: "claude-code", ModelBreakdown: []ModelUsageJSON{
			{Model: "claude-opus-4-5-20251101", Calls: 10},
			{Model: "claude-haiku-4-5-20251101", Calls: 5},
		},
	}
	findings := buildFindings([]AgentCacheResult{claude}, nil, fixer.Finding{}, false, i18n.FR)
	for _, f := range findings {
		if f.Label == copilotLabel {
			t.Fatalf("buildFindings() with a claude-code ModelBreakdown produced a Copilot-switch finding, want none: %+v", findings)
		}
	}

	copilot := AgentCacheResult{
		Agent: "copilot", ModelBreakdown: []ModelUsageJSON{
			{Model: "gpt-4o", Calls: 10},
			{Model: "gpt-4o-mini", Calls: 5},
		},
	}
	findings = buildFindings([]AgentCacheResult{copilot}, nil, fixer.Finding{}, false, i18n.FR)
	found := false
	for _, f := range findings {
		if f.Label == copilotLabel {
			found = true
		}
	}
	if !found {
		t.Fatalf("buildFindings() with a copilot ModelBreakdown did not produce the Copilot-switch finding, want one: %+v", findings)
	}
}

// TestDashboardSavedIsCostWithoutCacheMinusCost pins the card's
// explicit formula: saved = cost-without-cache - cost. Built from the
// same usdSplit/costWithoutCache/totalCacheUSD helpers `ats cache`
// itself uses -- no second cost model.
func TestDashboardSavedIsCostWithoutCacheMinusCost(t *testing.T) {
	a := AgentCacheResult{
		Agent: "claude-code", CacheReadTokens: 10_000_000, CacheWriteTokens: 1_000_000,
		InputTokens: 500_000, OutputTokens: 100_000,
	}
	a.USD, a.USDShare = usdSplit(a.InputTokens, a.OutputTokens, a.CacheReadTokens, a.CacheWriteTokens)

	got := dashboardSaved(a)
	want := costWithoutCache(a) - totalCacheUSD(a)
	if got != want {
		t.Fatalf("dashboardSaved = %v, want %v (costWithoutCache - totalCacheUSD)", got, want)
	}
	if got <= 0 {
		t.Fatalf("dashboardSaved = %v, want > 0 (cache reads are far cheaper than fresh input)", got)
	}
}

// TestDashboardSavedIsZeroWhenPriceUnknown: an agent with no pricing
// table entry (codex/copilot, CostUnknown=true, USD left empty) must
// report 0 saved, never an invented dollar figure.
func TestDashboardSavedIsZeroWhenPriceUnknown(t *testing.T) {
	a := AgentCacheResult{Agent: "codex", CostUnknown: true, CacheReadTokens: 5_000_000}
	if got := dashboardSaved(a); got != 0 {
		t.Fatalf("dashboardSaved(price unknown) = %v, want 0", got)
	}
}

// TestRankFindingsSortsBiggestMagnitudeFirst is the "A ameliorer"
// panel's ranking contract: biggest estimated impact first.
func TestRankFindingsSortsBiggestMagnitudeFirst(t *testing.T) {
	in := []dashboardFinding{
		{Label: "small", Magnitude: 1.5},
		{Label: "biggest", Magnitude: 42.0},
		{Label: "medium", Magnitude: 10.0},
	}
	got := rankFindings(in)
	want := []string{"biggest", "medium", "small"}
	for i, w := range want {
		if got[i].Label != w {
			t.Fatalf("rankFindings()[%d].Label = %q, want %q (full: %v)", i, got[i].Label, w, got)
		}
	}
}

// TestRankFindingsIsStableForEqualMagnitude: ties keep their original
// relative order instead of shuffling unpredictably between runs.
func TestRankFindingsIsStableForEqualMagnitude(t *testing.T) {
	in := []dashboardFinding{
		{Label: "first", Magnitude: 5},
		{Label: "second", Magnitude: 5},
	}
	got := rankFindings(in)
	if got[0].Label != "first" || got[1].Label != "second" {
		t.Fatalf("rankFindings() with equal magnitudes = %v, want order preserved", got)
	}
}

// TestBuildFindingsDedupesHermesFixAndTTLSavingForSameAgent pins card
// point 7: when the Hermes config fix finding (fixer.DetectHermesCacheTTLFinding)
// and a per-agent TTL saving finding (collectTTL's EstimatedSaving)
// both point at the *same* agent, they describe the same root problem
// ("this agent's cache TTL is badly tuned") twice -- dedupe by
// (agent, kind): drop the redundant per-agent TTL finding once the
// config-fix finding for that agent is already listed.
func TestBuildFindingsDedupesHermesFixAndTTLSavingForSameAgent(t *testing.T) {
	ttlFinding := fixer.Finding{Agent: "hermes", Key: fixer.HermesCacheTTLKey, OldValue: "1h", NewValue: "auto"}
	ttl := []AgentTTLResult{
		{Agent: "hermes", EstimatedSaving: 0.3, TotalWriteTokens: 1_000_000},
	}
	findings := buildFindings(nil, ttl, ttlFinding, true, i18n.FR)

	hermesTTLFindings := 0
	for _, f := range findings {
		if f.DetailKey == "4" || f.DetailKey == "5" {
			hermesTTLFindings++
		}
	}
	if hermesTTLFindings != 1 {
		t.Fatalf("buildFindings() with a Hermes fix finding and a Hermes TTL saving = %v, want exactly 1 TTL-related finding (deduped), got %d", findings, hermesTTLFindings)
	}
}

// TestBuildFindingsKeepsTTLSavingForADifferentAgent: the dedupe must
// only drop the *same*-agent duplicate, never a genuinely different
// agent's own TTL finding.
func TestBuildFindingsKeepsTTLSavingForADifferentAgent(t *testing.T) {
	ttlFinding := fixer.Finding{Agent: "hermes", Key: fixer.HermesCacheTTLKey, OldValue: "1h", NewValue: "auto"}
	ttl := []AgentTTLResult{
		{Agent: "claude-code", EstimatedSaving: 0.3, TotalWriteTokens: 1_000_000},
	}
	findings := buildFindings(nil, ttl, ttlFinding, true, i18n.FR)
	if len(findings) != 2 {
		t.Fatalf("buildFindings() with a Hermes fix finding and a claude-code TTL saving = %v, want both (2) findings kept", findings)
	}
}

// TestBuildFindingsLabelsAreShortPlainWords pins the card's explicit
// label budget ("short label (<= 40 chars, plain words)") across every
// finding type buildFindings can produce: the Hermes fix, a TTL
// saving, abnormal warm rebuilds, and a Copilot model switch.
func TestBuildFindingsLabelsAreShortPlainWords(t *testing.T) {
	cache := []AgentCacheResult{
		{
			Agent: "claude-code", WarmRebuilds: 1,
			WarmRebuildDetails: []WarmRebuildDetail{{Session: "s1", Timestamp: "t1", Tokens: 50_000}},
		},
		{
			Agent: "copilot", ModelBreakdown: []ModelUsageJSON{
				{Model: "gpt-4o", Calls: 10}, {Model: "gpt-4o-mini", Calls: 5},
			},
		},
	}
	ttl := []AgentTTLResult{
		{Agent: "hermes", EstimatedSaving: 0.3, TotalWriteTokens: 1_000_000},
	}
	findings := buildFindings(cache, ttl, fixer.Finding{}, true, i18n.FR)
	if len(findings) == 0 {
		t.Fatal("buildFindings returned no findings, want at least one per fixture")
	}
	for _, f := range findings {
		if n := len([]rune(f.Label)); n > 40 {
			t.Errorf("finding label %q is %d runes, want <= 40", f.Label, n)
		}
	}
}
