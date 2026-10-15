// Package copilotcli reads GitHub Copilot CLI's local usage database
// (~/.copilot/session-store.db, opened read-only) into the same cache
// read/write aggregate shape ats already computes for the other agents.
// Schema verified on a real Mac on 2026-10-14: table assistant_usage_events,
// columns session_id, model, input_tokens, output_tokens, cache_read_tokens,
// cache_write_tokens, reasoning_tokens, created_at (ISO 8601 UTC with ms).
package copilotcli

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// ModelUsage is one model's aggregated usage from assistant_usage_events.
// Copilot auto-routes requests across models, and a model switch empties
// the prompt cache, so usage is reported per model rather than as one
// blended total (see the card).
type ModelUsage struct {
	Model            string
	Sessions         int64 // distinct session_id
	Calls            int64
	InputTokens      int64 // cache-inclusive: see Uncached.
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

// Uncached returns InputTokens with CacheReadTokens subtracted:
// input_tokens is cache-inclusive, same convention as codexcli.
func (m ModelUsage) Uncached() int64 {
	return m.InputTokens - m.CacheReadTokens
}

// HitRate is CacheReadTokens / (Uncached + CacheReadTokens +
// CacheWriteTokens), the same formula used for every other agent.
func (m ModelUsage) HitRate() float64 {
	denom := m.Uncached() + m.CacheReadTokens + m.CacheWriteTokens
	if denom == 0 {
		return 0
	}
	return float64(m.CacheReadTokens) / float64(denom)
}

// isoCutoffFormat matches created_at's own format (ISO 8601 UTC with
// milliseconds), so a lexical string comparison in SQL is equivalent to a
// time comparison without needing sqlite date functions on this column.
const isoCutoffFormat = "2006-01-02T15:04:05.000Z"

// QueryByModel opens dbPath read-only (mode=ro: ats never writes to an
// agent's own database, even though session-store.db is WAL-mode) and
// groups assistant_usage_events by model, for events in the last `days`
// days relative to now.
func QueryByModel(dbPath string, days int, now time.Time) ([]ModelUsage, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	cutoff := now.AddDate(0, 0, -days).UTC().Format(isoCutoffFormat)
	const q = `
SELECT model, count(DISTINCT session_id), count(*),
       sum(input_tokens), sum(output_tokens), sum(reasoning_tokens),
       sum(cache_read_tokens), sum(cache_write_tokens)
FROM assistant_usage_events
WHERE created_at >= ?
GROUP BY model
ORDER BY model`
	rows, err := db.Query(q, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModelUsage
	for rows.Next() {
		var m ModelUsage
		var in, outT, reasoning, read, write sql.NullInt64
		if err := rows.Scan(&m.Model, &m.Sessions, &m.Calls, &in, &outT, &reasoning, &read, &write); err != nil {
			return nil, err
		}
		m.InputTokens, m.OutputTokens, m.ReasoningTokens = in.Int64, outT.Int64, reasoning.Int64
		m.CacheReadTokens, m.CacheWriteTokens = read.Int64, write.Int64
		out = append(out, m)
	}
	return out, rows.Err()
}
