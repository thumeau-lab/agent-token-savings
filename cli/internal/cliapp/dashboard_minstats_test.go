package cliapp

import "testing"

// TestBuildHeroMiniStatsComputesYesterdayAvgAndSwing pins the three
// hero mini-stats (card 1a) from a newest-first DayRows series.
func TestBuildHeroMiniStatsComputesYesterdayAvgAndSwing(t *testing.T) {
	rows := []dashboardDayRow{
		{Day: "2026-10-13", Cost: 11.90}, // yesterday
		{Day: "2026-10-12", Cost: 10.00}, // day before
		{Day: "2026-10-11", Cost: 9.00},
	}
	yesterday, yesterdayKnown, avg7, avg7Known, vsPct, vsKnown := buildHeroMiniStats(rows)
	if !yesterdayKnown || yesterday != 11.90 {
		t.Fatalf("yesterday = %v (known=%v), want 11.90", yesterday, yesterdayKnown)
	}
	wantAvg := (11.90 + 10.00 + 9.00) / 3
	if !avg7Known || diffAbs(avg7, wantAvg) > 1e-9 {
		t.Fatalf("avg7 = %v (known=%v), want %v", avg7, avg7Known, wantAvg)
	}
	wantPct := (11.90 - 10.00) / 10.00 * 100
	if !vsKnown || diffAbs(vsPct, wantPct) > 1e-9 {
		t.Fatalf("vsYesterdayPct = %v (known=%v), want %v", vsPct, vsKnown, wantPct)
	}
}

func diffAbs(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestBuildHeroMiniStatsNoDataKnown pins the "never invent a 0" rule:
// an empty series reports every *Known flag false.
func TestBuildHeroMiniStatsNoDataKnown(t *testing.T) {
	_, yesterdayKnown, _, avg7Known, _, vsKnown := buildHeroMiniStats(nil)
	if yesterdayKnown || avg7Known || vsKnown {
		t.Fatalf("expected every *Known flag false for an empty series")
	}
}

// TestBuildHeroMiniStatsSingleDayHasNoSwing: with only one day of
// history there is nothing to compare yesterday against.
func TestBuildHeroMiniStatsSingleDayHasNoSwing(t *testing.T) {
	rows := []dashboardDayRow{{Day: "2026-10-13", Cost: 5}}
	_, yesterdayKnown, _, _, _, vsKnown := buildHeroMiniStats(rows)
	if !yesterdayKnown {
		t.Fatal("yesterday should be known with one day of data")
	}
	if vsKnown {
		t.Fatal("vsYesterday should not be known with only one day of data")
	}
}
