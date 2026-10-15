package cliapp

import "testing"

// TestBuildEfficiencyHandComputedCases pins the card's formula with
// three hand-computed cases: score = 60% cache-hit-rate + 25%
// rebuild-free-rate + 15% TTL-well-tuned (0 or 100), rounded to the
// nearest integer, mapped to a letter grade.
func TestBuildEfficiencyHandComputedCases(t *testing.T) {
	cases := []struct {
		name       string
		hitRate    float64
		calls      int64
		rebuilds   int
		fixHasFind bool
		ttl        []AgentTTLResult
		wantScore  int
		wantLetter string
	}{
		{
			// 0.60*95 + 0.25*100 + 0.15*100 = 57 + 25 + 15 = 97
			name: "well tuned everywhere", hitRate: 0.95, calls: 100, rebuilds: 0,
			fixHasFind: false, wantScore: 97, wantLetter: "A+",
		},
		{
			// 0.60*70 + 0.25*80 + 0.15*0 = 42 + 20 + 0 = 62
			name: "misconfigured TTL and some rebuilds", hitRate: 0.70, calls: 100, rebuilds: 20,
			fixHasFind: true, wantScore: 62, wantLetter: "D",
		},
		{
			// 0.60*50 + 0.25*50 + 0.15*0 = 30 + 12.5 + 0 = 42.5 -> 43
			name: "poor cache and a worthwhile TTL finding", hitRate: 0.50, calls: 200, rebuilds: 100,
			fixHasFind: false, ttl: []AgentTTLResult{{Agent: "hermes", EstimatedSaving: 0.3}},
			wantScore: 43, wantLetter: "F",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cache := []AgentCacheResult{{Agent: "hermes", Calls: c.calls, WarmRebuilds: c.rebuilds}}
			got := buildEfficiency(cache, c.ttl, c.fixHasFind, "", c.hitRate)
			if got.Score != c.wantScore {
				t.Errorf("Score = %d, want %d", got.Score, c.wantScore)
			}
			if got.Letter != c.wantLetter {
				t.Errorf("Letter = %q, want %q", got.Letter, c.wantLetter)
			}
		})
	}
}

// TestEfficiencyLetterBoundaries pins the A+..F cutoffs themselves,
// independent of the weighted-score formula above.
func TestEfficiencyLetterBoundaries(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{100, "A+"}, {97, "A+"}, {96, "A"}, {93, "A"}, {90, "A-"},
		{89, "B+"}, {83, "B"}, {80, "B-"}, {79, "C+"}, {73, "C"},
		{70, "C-"}, {69, "D+"}, {60, "D"}, {59, "F"}, {0, "F"},
	}
	for _, c := range cases {
		if got := efficiencyLetter(c.score); got != c.want {
			t.Errorf("efficiencyLetter(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}
