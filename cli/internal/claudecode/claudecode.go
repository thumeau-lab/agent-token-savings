// Package claudecode ports tools/claude_cache_ratio.py: reading Claude Code's
// local JSONL transcripts (~/.claude/projects/*/*.jsonl) to measure prompt
// cache writes vs reads. See docs/levers.md section 2 for the lever this feeds.
package claudecode

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// UsageRow is one deduplicated `usage` block from a transcript line.
type UsageRow struct {
	TS     string
	Model  string
	Input  int64
	Output int64
	Read   int64
	Create int64
}

type dedupKey struct {
	input, output, read, create int64
}

// WarnCreate is the "big write" threshold for the resume signature: a cache
// write above this size, with no read, after the session has already gone
// warm once, flags an abnormal full rebuild.
const WarnCreate = 10_000

// BigCreate is the threshold for showing 'C' instead of '.' in a session shape.
const BigCreate = 2_000

// WarmRebuild is one occurrence of a warm session suddenly rewriting the
// whole prefix (create > WarnCreate, read == 0, after at least one real read).
type WarmRebuild struct {
	Session   string
	MsgIdx    int
	Create    int64
	Timestamp string
}

// SessionShape is the per-session summary used for the "worst cache breaks" view.
type SessionShape struct {
	Name   string
	Create int64
	Read   int64
	NCalls int
	Shape  string // "C" big write, "." small write, "_" read only, per call
}

// Totals aggregates token counts across every parsed session.
type Totals struct {
	Create int64
	Read   int64
	Input  int64
	Output int64
}

// HitRate is Read / (Read + Create), or 0 when there is nothing to divide.
func (t Totals) HitRate() float64 {
	denom := t.Create + t.Read
	if denom == 0 {
		return 0
	}
	return float64(t.Read) / float64(denom)
}

// Report is the full result of analyzing a root's transcripts, mirroring
// what tools/claude_cache_ratio.py prints.
type Report struct {
	Sessions     int
	Calls        int
	Totals       Totals
	TopSessions  []SessionShape
	FirstCold    int
	WarmRebuilds []WarmRebuild
	// ModelBreakdown groups usage by `message.model` (real Claude Code
	// assistant lines carry it): a model switch has the same prompt-cache
	// implication as for the other agents (see cliapp.AgentCacheResult.
	// ModelBreakdown's own doc), so the Dashboard's "Par modele" panel
	// needs this detail instead of one blended total. Sorted by model
	// name, same convention as copilotcli.QueryByModel's ORDER BY model.
	// Lines with no model field, or model "<synthetic>" (Claude Code's
	// own marker for lines with no real model inference -- card: "skip
	// <synthetic> model rows"), are excluded here but still counted in
	// Totals above: never silently dropped from the window's real numbers.
	ModelBreakdown []ModelUsage
}

// ModelUsage is one model's aggregated usage across every session.
type ModelUsage struct {
	Model  string
	Calls  int
	Totals Totals
}

// syntheticModel is Claude Code's own marker for an assistant line with
// no real model inference behind it (card: "skip <synthetic> model rows").
const syntheticModel = "<synthetic>"

// Aggregate walks root/projects/*/*.jsonl (oldest first, by mtime),
// keeps only the rows whose timestamp falls in [now-days, now] (card:
// "ats cache --days 1/7/30 print the same totals" -- claude-code used
// to ignore the window entirely), and builds a Report from what is
// left. A row whose timestamp cannot be parsed can never be placed in
// the window, so it is dropped rather than defaulted into it (same
// rule EstimateTTL already applies at the session level). Mirrors
// tools/claude_cache_ratio.py's main(), which has no such window.
func Aggregate(root string, days int, now time.Time) (Report, error) {
	paths, err := filepath.Glob(filepath.Join(root, "projects", "*", "*.jsonl"))
	if err != nil {
		return Report{}, err
	}
	sort.Slice(paths, func(i, j int) bool {
		ti := mtime(paths[i])
		tj := mtime(paths[j])
		return ti.Before(tj)
	})
	cutoff := now.AddDate(0, 0, -days)

	var rep Report
	byModel := map[string]*ModelUsage{}
	for _, p := range paths {
		rows, err := ParseSessionFile(p)
		if err != nil {
			return Report{}, err
		}
		rows = filterRowsSince(rows, cutoff)
		if len(rows) == 0 {
			continue
		}
		rep.Sessions++
		rep.Calls += len(rows)

		var c, r, in, out int64
		shape := make([]byte, 0, len(rows))
		for _, x := range rows {
			c += x.Create
			r += x.Read
			in += x.Input
			out += x.Output
			switch {
			case x.Create > BigCreate:
				shape = append(shape, 'C')
			case x.Create > 0:
				shape = append(shape, '.')
			default:
				shape = append(shape, '_')
			}
			if x.Model != "" && x.Model != syntheticModel {
				mu := byModel[x.Model]
				if mu == nil {
					mu = &ModelUsage{Model: x.Model}
					byModel[x.Model] = mu
				}
				mu.Calls++
				mu.Totals.Create += x.Create
				mu.Totals.Read += x.Read
				mu.Totals.Input += x.Input
				mu.Totals.Output += x.Output
			}
		}
		rep.Totals.Create += c
		rep.Totals.Read += r
		rep.Totals.Input += in
		rep.Totals.Output += out

		rep.TopSessions = append(rep.TopSessions, SessionShape{
			Name:   filepath.Base(p),
			Create: c,
			Read:   r,
			NCalls: len(rows),
			Shape:  string(shape),
		})

		if rows[0].Read == 0 {
			rep.FirstCold++
		}
		warm := false
		for idx, x := range rows {
			if idx == 0 {
				if x.Read > 0 {
					warm = true
				}
				continue
			}
			if warm && x.Read == 0 && x.Create > WarnCreate {
				rep.WarmRebuilds = append(rep.WarmRebuilds, WarmRebuild{
					Session: filepath.Base(p), MsgIdx: idx, Create: x.Create, Timestamp: x.TS,
				})
			}
			if x.Read > 0 {
				warm = true
			}
		}
	}

	sort.SliceStable(rep.TopSessions, func(i, j int) bool {
		return rep.TopSessions[i].Create > rep.TopSessions[j].Create
	})
	if len(rep.TopSessions) > 10 {
		rep.TopSessions = rep.TopSessions[:10]
	}

	for _, mu := range byModel {
		rep.ModelBreakdown = append(rep.ModelBreakdown, *mu)
	}
	sort.Slice(rep.ModelBreakdown, func(i, j int) bool {
		return rep.ModelBreakdown[i].Model < rep.ModelBreakdown[j].Model
	})
	return rep, nil
}

// filterRowsSince keeps only the rows whose timestamp parses and does
// not fall before cutoff; a row with no parsable timestamp is dropped
// (see Aggregate's doc comment).
func filterRowsSince(rows []UsageRow, cutoff time.Time) []UsageRow {
	out := make([]UsageRow, 0, len(rows))
	for _, r := range rows {
		ts, err := time.Parse(time.RFC3339, r.TS)
		if err != nil || ts.Before(cutoff) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func mtime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// ParseSessionFile reads one session's usage records, deduplicated.
//
// Some agents re-emit the same `usage` block on every event of a streamed
// turn; without dedup the totals get multiplied by the number of events.
// Mirrors tools/claude_cache_ratio.py's parse().
func ParseSessionFile(path string) ([]UsageRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[dedupKey]bool)
	var rows []UsageRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d struct {
			Timestamp string `json:"timestamp"`
			Message   struct {
				Model string `json:"model"`
				Usage *struct {
					InputTokens              int64 `json:"input_tokens"`
					OutputTokens             int64 `json:"output_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			continue
		}
		u := d.Message.Usage
		if u == nil {
			continue
		}
		key := dedupKey{u.InputTokens, u.OutputTokens, u.CacheReadInputTokens, u.CacheCreationInputTokens}
		if seen[key] {
			continue
		}
		seen[key] = true
		rows = append(rows, UsageRow{
			TS:     d.Timestamp,
			Model:  d.Message.Model,
			Input:  u.InputTokens,
			Output: u.OutputTokens,
			Read:   u.CacheReadInputTokens,
			Create: u.CacheCreationInputTokens,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}
