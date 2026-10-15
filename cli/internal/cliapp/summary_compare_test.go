package cliapp

import (
	"testing"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
)

// TestHermesComparisonNoPreviousData is the required "no previous data"
// case: an empty previous-window slice must report HasPrevious=false
// rather than a fabricated 0-point delta.
func TestHermesComparisonNoPreviousData(t *testing.T) {
	cur := []hermesusage.DailyRow{{Day: "2026-10-10", CacheRead: 100, CacheWrite: 10}}
	got := hermesComparison(cur, nil)
	if got.HasPrevious {
		t.Fatalf("HasPrevious = true, want false when there is no previous-window data")
	}
}

// TestHermesComparisonDeltaPointsUp hand-computes a 10-point hit-rate
// improvement: current window 90% (90/(90+10)), previous window 80%
// (80/(80+20)).
func TestHermesComparisonDeltaPointsUp(t *testing.T) {
	cur := []hermesusage.DailyRow{{Day: "2026-10-10", CacheRead: 90, CacheWrite: 10}}
	prev := []hermesusage.DailyRow{{Day: "2026-10-03", CacheRead: 80, CacheWrite: 20}}
	got := hermesComparison(cur, prev)
	if !got.HasPrevious {
		t.Fatalf("HasPrevious = false, want true")
	}
	if got.DeltaPoints < 9.9 || got.DeltaPoints > 10.1 {
		t.Fatalf("DeltaPoints = %v, want ~10", got.DeltaPoints)
	}
}

// TestSplitHermesWindowByDayCutoff pins the day-cutoff split: a row
// inside the last `days` days goes to current, a row older than that
// (but within the second `days`-sized window) goes to previous.
func TestSplitHermesWindowByDayCutoff(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	rows := []hermesusage.DailyRow{
		{Day: "2026-10-13"}, // within the last 7 days
		{Day: "2026-10-05"}, // in the previous 7-day window
	}
	cur, prev := splitHermesWindow(rows, 7, now)
	if len(cur) != 1 || cur[0].Day != "2026-10-13" {
		t.Fatalf("current = %v, want [2026-10-13]", cur)
	}
	if len(prev) != 1 || prev[0].Day != "2026-10-05" {
		t.Fatalf("previous = %v, want [2026-10-05]", prev)
	}
}

// TestClaudeComparisonNoPreviousData mirrors the Hermes no-previous-data
// case for Claude Code's own comparison function.
func TestClaudeComparisonNoPreviousData(t *testing.T) {
	cur := []claudecode.DailyRow{{Day: "2026-10-10", Read: 100, Create: 10}}
	got := claudeComparison(cur, nil)
	if got.HasPrevious {
		t.Fatalf("HasPrevious = true, want false when there is no previous-window data")
	}
}

// TestSplitClaudeWindowByDayCutoff pins the same cutoff logic over the
// full, unbounded DailyUsage history Claude Code returns.
func TestSplitClaudeWindowByDayCutoff(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	rows := []claudecode.DailyRow{
		{Day: "2026-10-13"}, // current window (last 7 days)
		{Day: "2026-10-05"}, // previous window (7-14 days ago)
		{Day: "2026-09-01"}, // older than the previous window: dropped
	}
	cur, prev := splitClaudeWindow(rows, 7, now)
	if len(cur) != 1 || cur[0].Day != "2026-10-13" {
		t.Fatalf("current = %v, want [2026-10-13]", cur)
	}
	if len(prev) != 1 || prev[0].Day != "2026-10-05" {
		t.Fatalf("previous = %v, want [2026-10-05]", prev)
	}
}
