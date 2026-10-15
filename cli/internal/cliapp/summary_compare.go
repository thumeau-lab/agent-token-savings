package cliapp

import (
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
)

// comparisonResult is the Summary tab's "compared to the previous
// period" finding: a hit-rate delta in percentage points between the
// current window and the window of equal length right before it, or
// HasPrevious=false when there is no previous-window data at all (the
// card: "if no previous data: say so", never an invented baseline).
type comparisonResult struct {
	HasPrevious bool
	DeltaPoints float64
}

// splitHermesWindow splits rows (the result of querying hermesusage'
// state.db for the last 2*days days) into the current window (the last
// `days` days) and the previous window of equal length right before it,
// by comparing each row's calendar day against the cutoff computed from
// now. No new parser: QueryDaily is the same reader `ats cache`/the
// Cache panel's sparkline already use, just called for a wider window
// and split in memory.
func splitHermesWindow(rows []hermesusage.DailyRow, days int, now time.Time) (current, previous []hermesusage.DailyRow) {
	cutoff := now.AddDate(0, 0, -days).Format("2006-01-02")
	for _, r := range rows {
		if r.Day >= cutoff {
			current = append(current, r)
		} else {
			previous = append(previous, r)
		}
	}
	return current, previous
}

// hermesComparison summarizes current/previous (already split by
// splitHermesWindow) through hermesusage.Summarize -- the same
// aggregator collectCache itself uses -- and reports the hit-rate
// delta in percentage points. HasPrevious is false when previous has no
// rows, so the caller never fabricates a 0-point "stable" verdict out of
// missing data.
func hermesComparison(current, previous []hermesusage.DailyRow) comparisonResult {
	if len(previous) == 0 {
		return comparisonResult{}
	}
	cur := hermesusage.Summarize(current)
	prev := hermesusage.Summarize(previous)
	return comparisonResult{HasPrevious: true, DeltaPoints: cur.HitRate*100 - prev.HitRate*100}
}

// splitClaudeWindow is splitHermesWindow's Claude Code counterpart:
// claudecode.DailyUsage returns every day in history (no window
// argument), so both cutoffs are computed here instead of asking for a
// wider query.
func splitClaudeWindow(rows []claudecode.DailyRow, days int, now time.Time) (current, previous []claudecode.DailyRow) {
	curCutoff := now.AddDate(0, 0, -days).Format("2006-01-02")
	prevCutoff := now.AddDate(0, 0, -2*days).Format("2006-01-02")
	for _, r := range rows {
		switch {
		case r.Day >= curCutoff:
			current = append(current, r)
		case r.Day >= prevCutoff:
			previous = append(previous, r)
		}
	}
	return current, previous
}

// claudeComparison mirrors hermesComparison using claudecode.Totals'
// own HitRate formula (Read / (Read + Create)) over the split rows.
func claudeComparison(current, previous []claudecode.DailyRow) comparisonResult {
	if len(previous) == 0 {
		return comparisonResult{}
	}
	hitRate := func(rows []claudecode.DailyRow) float64 {
		var read, create int64
		for _, r := range rows {
			read += r.Read
			create += r.Create
		}
		if denom := read + create; denom > 0 {
			return float64(read) / float64(denom)
		}
		return 0
	}
	return comparisonResult{HasPrevious: true, DeltaPoints: hitRate(current)*100 - hitRate(previous)*100}
}
