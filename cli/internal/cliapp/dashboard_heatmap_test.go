package cliapp

import (
	"testing"
	"time"
)

// TestBuildActivityHeatmapWeekAlignmentFixedClock pins the card's
// week/month alignment rule with a fixed clock: 7 rows (Mon..Sun),
// columns = weeks ending on the week containing `now`, and no data
// beyond `now` even when the week is not yet over.
func TestBuildActivityHeatmapWeekAlignmentFixedClock(t *testing.T) {
	now := time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC) // a Wednesday
	if now.Weekday() != time.Wednesday {
		t.Fatalf("fixture clock %v is not a Wednesday, fix the fixture", now)
	}
	h := buildActivityHeatmap(map[string]float64{"2026-10-14": 5, "2026-10-12": 2}, now, 12)

	if len(h.Weeks) != 12 {
		t.Fatalf("len(Weeks) = %d, want 12", len(h.Weeks))
	}
	lastWeek := h.Weeks[len(h.Weeks)-1]
	if lastWeek[0].Date != "2026-10-12" {
		t.Fatalf("lastWeek[0] (Monday) = %q, want 2026-10-12", lastWeek[0].Date)
	}
	if lastWeek[2].Date != "2026-10-14" {
		t.Fatalf("lastWeek[2] (Wednesday) = %q, want 2026-10-14 (now)", lastWeek[2].Date)
	}
	if !lastWeek[2].HasData || lastWeek[2].Cost != 5 {
		t.Fatalf("lastWeek[2] = %+v, want HasData cost 5", lastWeek[2])
	}
	if !lastWeek[0].HasData || lastWeek[0].Cost != 2 {
		t.Fatalf("lastWeek[0] = %+v, want HasData cost 2", lastWeek[0])
	}
	for i := 3; i < 7; i++ {
		if lastWeek[i].HasData {
			t.Fatalf("lastWeek[%d] = %+v, want no data (day is after `now`)", i, lastWeek[i])
		}
	}
	if h.ActiveDays != 2 {
		t.Fatalf("ActiveDays = %d, want 2", h.ActiveDays)
	}
}

// TestBuildActivityHeatmapMonthLabelsMarkMonthStart: a 12-week window
// spans more than one calendar month, so at least one column must
// carry a non-empty month label.
func TestBuildActivityHeatmapMonthLabelsMarkMonthStart(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	h := buildActivityHeatmap(map[string]float64{}, now, 12)
	nonEmpty := 0
	for _, m := range h.MonthStarts {
		if m != 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Fatal("expected at least one month marker across a 12-week window spanning multiple months")
	}
	if len(h.MonthStarts) != len(h.Weeks) {
		t.Fatalf("len(MonthStarts) = %d, want %d (one per week column)", len(h.MonthStarts), len(h.Weeks))
	}
}

// TestHeatmapShadeBucketsByCostQuartile pins the 4-shade-plus-empty
// bucketing rule the card requires ("4 orange shades by daily cost +
// empty dim cell").
func TestHeatmapShadeBucketsByCostQuartile(t *testing.T) {
	cases := []struct {
		cost, max float64
		want      int
	}{
		{0, 10, -1},
		{1, 10, 0},
		{4, 10, 1},
		{6, 10, 2},
		{9, 10, 3},
		{0, 0, -1},
	}
	for _, c := range cases {
		if got := heatmapShade(c.cost, c.max); got != c.want {
			t.Errorf("heatmapShade(%v, %v) = %d, want %d", c.cost, c.max, got, c.want)
		}
	}
}
