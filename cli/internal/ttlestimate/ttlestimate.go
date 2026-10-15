// Package ttlestimate ports tools/ttl_auto_estimate.py: estimating the saving
// of switching machine sessions (subagent, cron, oneshot...) from a 1h cache
// TTL to an automatic 5m/1h choice, based on the real gap between messages.
// See docs/levers.md section 3, `prompt_caching.cache_ttl: auto`.
package ttlestimate

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// machineSources mirrors ttl_auto_estimate.py's MACHINE set: sessions started
// by something other than an interactive human, where a long TTL is wasted if
// the calls are far apart.
var machineSources = map[string]bool{
	"subagent": true, "cron": true, "oneshot": true, "webhook": true,
	"kanban": true, "api": true, "tool": true, "batch": true,
}

// gapThresholdSeconds: a gap longer than this means a 5m TTL would have lost
// the cache and rewritten the prefix (extra cost). Mirrors ttl_auto_estimate.py.
const gapThresholdSeconds = 300

// write premium multipliers relative to base input price (docs/levers.md).
const (
	write5mMultiplier = 1.25
	write1hMultiplier = 2.0
)

// savingRatio5mOver1h is (1h - 5m) / 1h = (2 - 1.25) / 2 = 0.375, the fraction
// of a write's cost saved by using 5m TTL instead of 1h, when the cache would
// have stayed warm anyway. Mirrors the literal 0.375 in ttl_auto_estimate.py.
const savingRatio5mOver1h = (write1hMultiplier - write5mMultiplier) / write1hMultiplier

// Result is the aggregate computed over every session database under home.
type Result struct {
	MachineSessions    int64
	GapSessions        int64 // machine sessions with a gap > 5 min
	MachineWriteTokens int64
	TotalWriteTokens   int64 // across every session, any source
	GapWriteTokens     int64 // write tokens from machine sessions with gap > 5min
}

// NoGapWriteTokens is the write volume from machine sessions that never had a
// gap over 5 minutes: switching them to 5m TTL loses nothing but the premium.
func (r Result) NoGapWriteTokens() int64 {
	return r.MachineWriteTokens - r.GapWriteTokens
}

// EstimatedSavingRatio is the lower-bound fraction of TOTAL write cost saved
// by switching no-gap machine sessions to 5m TTL. Mirrors ttl_auto_estimate.py's
// final line; sessions with gaps are not counted (lower bound).
func (r Result) EstimatedSavingRatio() float64 {
	wa := r.TotalWriteTokens
	if wa == 0 {
		wa = 1
	}
	return float64(r.NoGapWriteTokens()) * savingRatio5mOver1h / float64(wa)
}

// Format renders the same 4 lines as tools/ttl_auto_estimate.py, in the same
// units and rounding, so the golden test can compare stdout byte for byte.
func (r Result) Format() string {
	w := float64(r.MachineWriteTokens)
	wa := float64(r.TotalWriteTokens)
	if wa == 0 {
		wa = 1
	}
	noGap := float64(r.NoGapWriteTokens())
	return fmt.Sprintf(
		"machine sessions=%d  of which gap>5min=%d\n"+
			"machine writes=%.1fM of %.1fM total (%.0f%%)\n"+
			"sessions without gap>5min: writes %.1fM, saving on these with 5m = 37.5%%\n"+
			"estimated saving on total writes: %.1f%% (lower bound, sessions with gaps not counted)\n",
		r.MachineSessions, r.GapSessions,
		w/1e6, wa/1e6, w/wa*100,
		noGap/1e6,
		r.EstimatedSavingRatio()*100,
	)
}

// Estimate reads home/state.db and home/profiles/*/state.db (read-only) and
// aggregates machine sessions from the last `days` days. Mirrors
// tools/ttl_auto_estimate.py's main loop (default 30 days there).
func Estimate(home string, days int) (Result, error) {
	dbs := []string{filepath.Join(home, "state.db")}
	profileDBs, err := filepath.Glob(filepath.Join(home, "profiles", "*", "state.db"))
	if err != nil {
		return Result{}, err
	}
	dbs = append(dbs, profileDBs...)

	var res Result
	sinceClause := fmt.Sprintf("-%d days", days)

	for _, dbPath := range dbs {
		db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
		if err != nil {
			continue // absent db: skip cleanly, like glob() finding nothing in Python
		}
		rows, err := db.Query(
			`select id, source, coalesce(cache_write_tokens,0) from sessions where started_at > unixepoch('now', ?)`,
			sinceClause)
		if err != nil {
			db.Close()
			continue
		}
		type sess struct {
			id, source string
			write      int64
		}
		var list []sess
		for rows.Next() {
			var s sess
			var source sql.NullString
			if err := rows.Scan(&s.id, &source, &s.write); err != nil {
				rows.Close()
				db.Close()
				return Result{}, err
			}
			s.source = source.String
			list = append(list, s)
		}
		rows.Close()

		for _, s := range list {
			res.TotalWriteTokens += s.write
			if !machineSources[s.source] || s.write == 0 {
				continue
			}
			gap, err := maxGap(db, s.id)
			if err != nil {
				db.Close()
				return Result{}, err
			}
			res.MachineSessions++
			res.MachineWriteTokens += s.write
			if gap > gapThresholdSeconds {
				res.GapSessions++
				res.GapWriteTokens += s.write
			}
		}
		db.Close()
	}
	return res, nil
}

func maxGap(db *sql.DB, sessionID string) (float64, error) {
	rows, err := db.Query(`select timestamp from messages where session_id=? order by timestamp`, sessionID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var prev float64
	var hasPrev bool
	var maxG float64
	for rows.Next() {
		var ts float64
		if err := rows.Scan(&ts); err != nil {
			return 0, err
		}
		if hasPrev {
			if gap := ts - prev; gap > maxG {
				maxG = gap
			}
		}
		prev = ts
		hasPrev = true
	}
	return maxG, rows.Err()
}
