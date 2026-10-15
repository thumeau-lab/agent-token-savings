// Package claudecode: 5m-vs-1h cache TTL cost estimate for Claude Code
// sessions, computed directly from the gap preceding each call (ats ttl
// claude-code), not from tools/ttl_auto_estimate.py's machine/human
// session split.
//
// That split does not transfer here: ttl_auto_estimate.py (and
// internal/ttlestimate, applied to Hermes) separates MACHINE sessions
// (subagent, cron, oneshot...) from human-interactive ones, because only
// machine sessions can go idle long enough, unattended, to lose a 5m
// cache. Claude Code's local transcripts carry no such distinction --
// every session here is a human driving an interactive CLI, so "which
// sessions are machine sessions" is not a question that applies. Instead,
// every call's own gap decides its cache fate under each TTL:
//   - the first call of a session always writes (cold, nothing to hit yet)
//   - a gap <=5min is a cache hit under BOTH a 5m and a 1h TTL
//   - a gap in (5min, 1h] is a hit under a 1h TTL, but a forced rewrite
//     (miss) under a 5m TTL -- this is the only bucket a TTL choice can
//     avoid
//   - a gap >1h is a miss under EITHER TTL: the rewrite happens
//     regardless, only its per-token price changes
//
// See docs/levers.md section 2 for the lever this feeds, and
// tools/ttl_auto_estimate.py for the method this intentionally diverges
// from for this agent.
package claudecode

import (
	"path/filepath"
	"sort"
	"time"
)

// gapShortThresholdSeconds / gapLongThresholdSeconds bound the three call
// buckets: <=5min (hit under either TTL), (5min,1h] (hit only under 1h),
// >1h (miss under either TTL).
const (
	gapShortThresholdSeconds = 300
	gapLongThresholdSeconds  = 3600
)

// write price multipliers relative to base input price (docs/levers.md),
// same figures as internal/ttlestimate.
const (
	write5mMultiplier = 1.25
	write1hMultiplier = 2.0
)

// TTLResult is the aggregate computed over every session transcript under
// root whose last activity falls within the requested days window, each
// call bucketed by the gap preceding it.
type TTLResult struct {
	Sessions int64
	Calls    int64

	// FirstCallWriteTokens is written by the first call of every counted
	// session: always a write, regardless of TTL choice.
	FirstCallWriteTokens int64
	// ShortGapWriteTokens/ShortGapCalls: calls after a gap <=5min, a hit
	// under either TTL. Kept for the total and for display, not used in
	// the cost comparison (neither TTL choice affects them).
	ShortGapWriteTokens int64
	ShortGapCalls       int64
	// MidGapWriteTokens/MidGapCalls: calls after a gap in (5min,1h] -- a
	// forced rewrite under a 5m TTL, but a cache hit under a 1h TTL. The
	// only bucket a TTL choice can avoid.
	MidGapWriteTokens int64
	MidGapCalls       int64
	// LongGapWriteTokens/LongGapCalls: calls after a gap >1h -- a miss
	// under either TTL; only the per-token write price changes.
	LongGapWriteTokens int64
	LongGapCalls       int64
}

// TotalWriteTokens is the sum of every bucket: the write volume actually
// observed, independent of any TTL choice.
func (r TTLResult) TotalWriteTokens() int64 {
	return r.FirstCallWriteTokens + r.ShortGapWriteTokens + r.MidGapWriteTokens + r.LongGapWriteTokens
}

// Cost5m is the relative cost (base-price units) of writing every call
// that is NOT a hit under an always-5m TTL: the first call, every
// mid-gap call (forced rewrite under 5m) and every long-gap call (a miss
// regardless), each at the 5m write multiplier.
func (r TTLResult) Cost5m() float64 {
	return float64(r.FirstCallWriteTokens+r.MidGapWriteTokens+r.LongGapWriteTokens) * write5mMultiplier
}

// Cost1h is the same comparison under an always-1h TTL: mid-gap calls
// become hits (no write at all), so only the first call and long-gap
// calls write, at the pricier 1h multiplier.
func (r TTLResult) Cost1h() float64 {
	return float64(r.FirstCallWriteTokens+r.LongGapWriteTokens) * write1hMultiplier
}

// SavingRatio is the fraction of Cost5m saved by switching to a 1h TTL.
// Positive when 1h is cheaper overall (enough mid-gap rewrites avoided to
// outweigh the pricier multiplier on calls that write either way);
// zero or negative when it is not worth it. Mirrors the shape of
// internal/ttlestimate.Result.EstimatedSavingRatio() without inheriting
// its machine-session precondition, which does not apply here.
func (r TTLResult) SavingRatio() float64 {
	cost5m := r.Cost5m()
	if cost5m == 0 {
		return 0
	}
	return (cost5m - r.Cost1h()) / cost5m
}

// EstimateTTL walks root/projects/*/*.jsonl, keeps sessions whose last
// activity falls within the last `days` days (mirrors the --days flag
// shared with cache/ttl for the other agents), and buckets every call by
// the gap preceding it (see the package doc comment). now is the
// injected clock the window is computed against (card: use the same
// injected-clock rule as every other window computation, never the
// real wall clock directly).
func EstimateTTL(root string, days int, now time.Time) (TTLResult, error) {
	paths, err := filepath.Glob(filepath.Join(root, "projects", "*", "*.jsonl"))
	if err != nil {
		return TTLResult{}, err
	}
	sort.Strings(paths)

	cutoff := now.AddDate(0, 0, -days)

	var res TTLResult
	for _, p := range paths {
		rows, err := ParseSessionFile(p)
		if err != nil {
			return TTLResult{}, err
		}
		if len(rows) == 0 {
			continue
		}
		if last, ok := lastParsedTimestamp(rows); ok && last.Before(cutoff) {
			continue
		}
		res.Sessions++

		var prev time.Time
		hasPrev := false
		for _, row := range rows {
			res.Calls++
			ts, err := time.Parse(time.RFC3339, row.TS)
			if err != nil {
				// No usable timestamp: can't place it relative to a
				// previous call, so treat it like a cold/first write.
				res.FirstCallWriteTokens += row.Create
				continue
			}
			if !hasPrev {
				res.FirstCallWriteTokens += row.Create
			} else {
				switch gap := ts.Sub(prev).Seconds(); {
				case gap <= gapShortThresholdSeconds:
					res.ShortGapWriteTokens += row.Create
					res.ShortGapCalls++
				case gap <= gapLongThresholdSeconds:
					res.MidGapWriteTokens += row.Create
					res.MidGapCalls++
				default:
					res.LongGapWriteTokens += row.Create
					res.LongGapCalls++
				}
			}
			prev = ts
			hasPrev = true
		}
	}
	return res, nil
}

// lastParsedTimestamp returns the last row whose timestamp parses, so the
// days-window check can be based on a session's most recent known
// activity even if its last line or two carries an unparsable timestamp.
func lastParsedTimestamp(rows []UsageRow) (time.Time, bool) {
	for i := len(rows) - 1; i >= 0; i-- {
		if ts, err := time.Parse(time.RFC3339, rows[i].TS); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}
