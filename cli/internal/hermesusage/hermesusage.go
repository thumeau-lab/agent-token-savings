// Package hermesusage ports tools/hermes_usage.py: aggregating Hermes'
// ~/.hermes/state.db (read-only) sessions table into cache/output cost shares.
// See docs/levers.md section 3 for the lever this feeds.
package hermesusage

import (
	"database/sql"
	"strconv"

	_ "modernc.org/sqlite"
)

// PriceUSDPerMTok are $/MTok, Opus 5.5, cache write with 1h TTL.
// https://platform.claude.com/docs/en/build-with-claude/prompt-caching#pricing (read 2026-10-11)
var PriceUSDPerMTok = map[string]float64{
	"input":       4.0,
	"output":      20.0,
	"cache_write": 8.0,
	"cache_read":  0.20,
}

// DailyRow is one aggregated day of sessions, mirroring hermes_usage.py's DAILY query.
type DailyRow struct {
	Day        string
	Sessions   int64
	Calls      int64
	Output     int64
	Input      int64
	CacheRead  int64
	CacheWrite int64
}

// PerCall divides tok by calls, returning 0 instead of dividing by zero.
func PerCall(tok, calls int64) int64 {
	if calls == 0 {
		return 0
	}
	return tok / calls
}

// Summary is the aggregate over a set of DailyRow, mirroring summarize().
type Summary struct {
	Sessions       int64
	Calls          int64
	Output         int64
	Input          int64
	CacheRead      int64
	CacheWrite     int64
	ReadPerCall    int64
	WritePerCall   int64
	HitRate        float64 // cache_read / (cache_read + cache_write)
	ReadOverOutput float64 // cache_read / output
	USD            map[string]float64
	USDShare       map[string]float64
}

// Summarize aggregates rows into totals, ratios and the weighted USD split.
// Mirrors tools/hermes_usage.py's summarize().
func Summarize(rows []DailyRow) Summary {
	var s Summary
	s.USD = map[string]float64{}
	s.USDShare = map[string]float64{}

	tok := map[string]int64{}
	for _, r := range rows {
		s.Sessions += r.Sessions
		s.Calls += r.Calls
		tok["output"] += r.Output
		tok["input"] += r.Input
		tok["cache_read"] += r.CacheRead
		tok["cache_write"] += r.CacheWrite
	}
	s.Output, s.Input, s.CacheRead, s.CacheWrite = tok["output"], tok["input"], tok["cache_read"], tok["cache_write"]

	var total float64
	for k, v := range tok {
		usd := float64(v) * PriceUSDPerMTok[k] / 1e6
		s.USD[k] = usd
		total += usd
	}
	for k, v := range s.USD {
		if total != 0 {
			s.USDShare[k] = v / total
		}
	}

	s.ReadPerCall = PerCall(s.CacheRead, s.Calls)
	s.WritePerCall = PerCall(s.CacheWrite, s.Calls)
	if denom := s.CacheRead + s.CacheWrite; denom != 0 {
		s.HitRate = float64(s.CacheRead) / float64(denom)
	}
	if s.Output != 0 {
		s.ReadOverOutput = float64(s.CacheRead) / float64(s.Output)
	}
	return s
}

// QueryDaily reads daily aggregates from a Hermes state.db for the last
// `days` days, mirroring hermes_usage.py's DAILY query. The db is opened
// read-only; ats never writes to an agent's own state database.
func QueryDaily(dbPath string, days int) ([]DailyRow, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	const q = `
SELECT date(started_at,'unixepoch','localtime') d, count(*) n,
       sum(api_call_count) calls, sum(output_tokens) out_t, sum(input_tokens) in_t,
       sum(cache_read_tokens) cr, sum(cache_write_tokens) cw
FROM sessions WHERE started_at > unixepoch('now', ?)
GROUP BY d ORDER BY d`

	win := "-" + strconv.Itoa(days) + " days"
	rows, err := db.Query(q, win)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DailyRow
	for rows.Next() {
		var r DailyRow
		var calls, outT, inT, cr, cw sql.NullInt64
		if err := rows.Scan(&r.Day, &r.Sessions, &calls, &outT, &inT, &cr, &cw); err != nil {
			return nil, err
		}
		r.Calls, r.Output, r.Input, r.CacheRead, r.CacheWrite = calls.Int64, outT.Int64, inT.Int64, cr.Int64, cw.Int64
		out = append(out, r)
	}
	return out, rows.Err()
}
