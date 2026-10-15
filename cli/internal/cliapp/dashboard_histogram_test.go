package cliapp

import (
	"testing"
	"time"
)

// TestBuildHistogramBarsCoverWindowChronologically pins the card's
// "Depenses quotidiennes" data: exactly `days` bars, oldest first
// (left-to-right reading), ending on `now`'s day.
func TestBuildHistogramBarsCoverWindowChronologically(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	dayCost := map[string]float64{
		"2026-10-12": 3,
		"2026-10-13": 5,
		"2026-10-14": 2,
	}
	h := buildHistogram(dayCost, now, 7)
	if len(h.Bars) != 7 {
		t.Fatalf("len(Bars) = %d, want 7", len(h.Bars))
	}
	if h.Bars[0].Day != "2026-10-08" {
		t.Fatalf("Bars[0].Day = %q, want 2026-10-08 (oldest of a 7-day window ending 2026-10-14)", h.Bars[0].Day)
	}
	if h.Bars[6].Day != "2026-10-14" {
		t.Fatalf("Bars[6].Day = %q, want 2026-10-14 (now)", h.Bars[6].Day)
	}
	if h.Bars[6].Cost != 2 {
		t.Fatalf("Bars[6].Cost = %v, want 2", h.Bars[6].Cost)
	}
}

// TestBuildHistogramScale pins the headline chips: Moy./jour, Pic (+
// its date), and Hier.
func TestBuildHistogramScale(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	dayCost := map[string]float64{
		"2026-10-12": 3,
		"2026-10-13": 12, // peak
		"2026-10-14": 2,  // yesterday relative to "now" is 10-13; but "Bars[6]"=now's day itself
	}
	h := buildHistogram(dayCost, now, 3)
	if h.Peak != 12 {
		t.Fatalf("Peak = %v, want 12", h.Peak)
	}
	if h.PeakDay != "2026-10-13" {
		t.Fatalf("PeakDay = %q, want 2026-10-13", h.PeakDay)
	}
	wantAvg := (3.0 + 12.0 + 2.0) / 3
	if h.AvgPerDay != wantAvg {
		t.Fatalf("AvgPerDay = %v, want %v", h.AvgPerDay, wantAvg)
	}
	if !h.YesterdayKnown || h.Yesterday != 2 {
		t.Fatalf("Yesterday = %v (known=%v), want 2 (the last bar, `now`'s day)", h.Yesterday, h.YesterdayKnown)
	}
}

// TestBuildHistogramEmptyWindowHasNoPeakOrYesterday: an all-zero
// window must never invent a yesterday/peak day.
func TestBuildHistogramEmptyWindowHasNoPeakOrYesterday(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	h := buildHistogram(map[string]float64{}, now, 7)
	if h.Peak != 0 || h.PeakDay != "" {
		t.Fatalf("Peak = %v PeakDay = %q, want 0 and empty for an all-zero window", h.Peak, h.PeakDay)
	}
	if !h.YesterdayKnown {
		t.Fatal("Yesterday should still be known (0) when the window itself is non-empty, just all-zero cost")
	}
	if h.Yesterday != 0 {
		t.Fatalf("Yesterday = %v, want 0", h.Yesterday)
	}
}
