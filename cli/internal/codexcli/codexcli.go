// Package codexcli ports Codex CLI's local session transcripts
// (`$CODEX_HOME` or `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`, plus
// `~/.codex/archived_sessions/`) into the same cache read/write aggregate
// shape ats already computes for claude-code and hermes. Schema verified
// on a real Mac on 2026-10-14 against codex-cli 0.162.1 (see the card).
package codexcli

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// LastTokenUsage is one event_msg/token_count payload.info.last_token_usage
// block (codex-cli 0.162.1). CacheWriteInputTokens is a pointer because
// older codex-cli versions never emit the field at all: nil must never be
// treated the same as 0 (the card: "report write as n/a ... never 0").
type LastTokenUsage struct {
	InputTokens           int64  `json:"input_tokens"`
	CachedInputTokens     int64  `json:"cached_input_tokens"`
	CacheWriteInputTokens *int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64  `json:"output_tokens"`
	ReasoningOutputTokens int64  `json:"reasoning_output_tokens"`
	TotalTokens           int64  `json:"total_tokens"`
}

// UsageRow is one deduplicated last_token_usage snapshot from a rollout file.
type UsageRow struct {
	Input           int64
	CachedInput     int64
	CacheWrite      *int64 // nil when cache_write_input_tokens is absent: never 0.
	Output          int64
	ReasoningOutput int64
	// Model is the model active at the time of this event, from the
	// most recent "turn_context" line seen before it in the same
	// rollout file (codex-rs protocol.rs TurnContextItem.model): "" if
	// no turn_context line has appeared yet, never invented.
	Model string
}

// FileResult is one rollout/archived session file's parsed usage rows plus
// the metadata Aggregate needs to build the "sessions without usage" count
// and the n/a-vs-zero cache-write distinction.
type FileResult struct {
	Rows []UsageRow
	// SawTokenUsageRecord is true when a newer "token_usage_record" event
	// type was found in the file: per the card, token_count is ignored
	// entirely for such a file (see the TODO below), so Rows is always
	// empty when this is true.
	SawTokenUsageRecord bool
}

type eventMsgLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type string `json:"type"`
		Info *struct {
			LastTokenUsage *LastTokenUsage `json:"last_token_usage"`
		} `json:"info"`
		// Model is set when Type (outer) == "turn_context": the model
		// selected for that turn (codex-rs protocol.rs
		// TurnContextItem.model). Absent/empty on an event_msg line.
		Model string `json:"model"`
	} `json:"payload"`
}

// equalUsage reports whether two LastTokenUsage values are identical
// (including CacheWriteInputTokens's presence/value, dereferenced), for
// ParseSessionFile's consecutive-duplicate dedupe.
func equalUsage(a, b LastTokenUsage) bool {
	if a.InputTokens != b.InputTokens || a.CachedInputTokens != b.CachedInputTokens ||
		a.OutputTokens != b.OutputTokens || a.ReasoningOutputTokens != b.ReasoningOutputTokens ||
		a.TotalTokens != b.TotalTokens {
		return false
	}
	switch {
	case a.CacheWriteInputTokens == nil && b.CacheWriteInputTokens == nil:
		return true
	case a.CacheWriteInputTokens == nil || b.CacheWriteInputTokens == nil:
		return false
	default:
		return *a.CacheWriteInputTokens == *b.CacheWriteInputTokens
	}
}

// ParseSessionFile reads one rollout/archived session file's token_count
// events.
//
// info can be null (interactive TUI sessions that never emitted a
// token_count event upstream: openai/codex#9660): those lines are
// skipped, not treated as an error. Consecutive identical
// last_token_usage blocks are deduplicated (codex-cli's own known
// duplicate emission).
//
// TODO(codexcli): a newer "token_usage_record" event type exists in some
// files (see the card); its field names were not available on any real
// fixture at the time this was written, so a file containing one is only
// flagged via SawTokenUsageRecord and its token_count events are ignored
// entirely (per the card) -- do not invent the newer schema. Revisit once
// a real fixture turns up.
func ParseSessionFile(path string) (FileResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileResult{}, err
	}
	defer f.Close()

	var res FileResult
	var prev *LastTokenUsage
	var currentModel string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var d eventMsgLine
		if err := json.Unmarshal(line, &d); err != nil {
			continue
		}
		if d.Type == "turn_context" {
			if d.Payload.Model != "" {
				currentModel = d.Payload.Model
			}
			continue
		}
		if d.Type != "event_msg" {
			continue
		}
		switch d.Payload.Type {
		case "token_usage_record":
			res.SawTokenUsageRecord = true
		case "token_count":
			if d.Payload.Info == nil || d.Payload.Info.LastTokenUsage == nil {
				continue
			}
			u := d.Payload.Info.LastTokenUsage
			if prev != nil && equalUsage(*prev, *u) {
				continue
			}
			prev = u
			res.Rows = append(res.Rows, UsageRow{
				Input: u.InputTokens, CachedInput: u.CachedInputTokens,
				CacheWrite: u.CacheWriteInputTokens, Output: u.OutputTokens,
				ReasoningOutput: u.ReasoningOutputTokens, Model: currentModel,
			})
		}
	}
	if err := sc.Err(); err != nil {
		return FileResult{}, err
	}
	if res.SawTokenUsageRecord {
		res.Rows = nil
	}
	return res, nil
}

// Totals aggregates token counts across every parsed rollout file.
type Totals struct {
	Input           int64 // cache-inclusive (see Uncached's comment).
	CachedInput     int64
	Output          int64
	ReasoningOutput int64
	CacheWrite      int64
	// CacheWriteUnknown is true when at least one contributing row had no
	// cache_write_input_tokens field at all (older codex-cli): the card
	// requires showing "n/a" for cache write in that case, never 0.
	CacheWriteUnknown bool
}

// Uncached returns Input with CachedInput subtracted.
//
// ponytail: OpenAI documents input_tokens as already counting
// cached_input_tokens inside it. Whether cache_write_input_tokens is also
// double-counted inside input_tokens cannot be checked on a fixture-free
// basis (no real sample was available either way), so it is treated as
// NOT included in Input -- the smallest assumption consistent with
// OpenAI's documented behaviour. If that assumption is ever wrong,
// Uncached overcounts by CacheWrite.
func (t Totals) Uncached() int64 {
	return t.Input - t.CachedInput
}

// HitRate is CachedInput / (Uncached + CachedInput + CacheWrite), the
// same formula used for every other agent (read / (uncached input + read
// + write)).
func (t Totals) HitRate() float64 {
	denom := t.Uncached() + t.CachedInput + t.CacheWrite
	if denom == 0 {
		return 0
	}
	return float64(t.CachedInput) / float64(denom)
}

// Report is the full result of analyzing a Codex home's session files.
type Report struct {
	Sessions int // files that yielded at least one usage row
	Calls    int // total token_count events across sessions (post-dedupe)
	Totals   Totals
	// SessionsWithoutUsage counts rollout files found with zero usable
	// token_count events (null info throughout, or a file where every
	// token_count was ignored because of a token_usage_record event):
	// interactive-only sessions (openai/codex#9660) are counted here, not
	// silently dropped.
	SessionsWithoutUsage int
	// ModelBreakdown groups usage by the model active at each event
	// (see UsageRow.Model, from "turn_context" lines): a model switch
	// has the same prompt-cache implication as for the other agents
	// (see cliapp.AgentCacheResult.ModelBreakdown's own doc), so the
	// Dashboard's "Par modele" panel needs this detail instead of one
	// blended total. Rows with no model known yet (Model == "") are
	// excluded here but still counted in Totals above: never silently
	// dropped from the window's real numbers.
	ModelBreakdown []ModelUsage
}

// ModelUsage is one model's aggregated usage across every session.
type ModelUsage struct {
	Model  string
	Calls  int
	Totals Totals
}

// Aggregate walks root/sessions/*/*/*/rollout-*.jsonl and every *.jsonl
// under root/archived_sessions, building a Report.
func Aggregate(root string) (Report, error) {
	paths, err := sessionFiles(root)
	if err != nil {
		return Report{}, err
	}
	sort.Strings(paths)

	var rep Report
	byModel := map[string]*ModelUsage{}
	for _, p := range paths {
		fr, err := ParseSessionFile(p)
		if err != nil {
			return Report{}, err
		}
		if len(fr.Rows) == 0 {
			rep.SessionsWithoutUsage++
			continue
		}
		rep.Sessions++
		rep.Calls += len(fr.Rows)
		for _, row := range fr.Rows {
			rep.Totals.Input += row.Input
			rep.Totals.CachedInput += row.CachedInput
			rep.Totals.Output += row.Output
			rep.Totals.ReasoningOutput += row.ReasoningOutput
			if row.CacheWrite == nil {
				rep.Totals.CacheWriteUnknown = true
			} else {
				rep.Totals.CacheWrite += *row.CacheWrite
			}
			if row.Model != "" {
				mu := byModel[row.Model]
				if mu == nil {
					mu = &ModelUsage{Model: row.Model}
					byModel[row.Model] = mu
				}
				mu.Calls++
				mu.Totals.Input += row.Input
				mu.Totals.CachedInput += row.CachedInput
				mu.Totals.Output += row.Output
				mu.Totals.ReasoningOutput += row.ReasoningOutput
				if row.CacheWrite == nil {
					mu.Totals.CacheWriteUnknown = true
				} else {
					mu.Totals.CacheWrite += *row.CacheWrite
				}
			}
		}
	}
	for _, mu := range byModel {
		rep.ModelBreakdown = append(rep.ModelBreakdown, *mu)
	}
	sort.Slice(rep.ModelBreakdown, func(i, j int) bool {
		return rep.ModelBreakdown[i].Model < rep.ModelBreakdown[j].Model
	})
	return rep, nil
}

// sessionFiles lists every rollout-*.jsonl under root/sessions/*/*/* plus
// every *.jsonl under root/archived_sessions, at any nesting depth.
func sessionFiles(root string) ([]string, error) {
	var out []string
	matches, err := filepath.Glob(filepath.Join(root, "sessions", "*", "*", "*", "rollout-*.jsonl"))
	if err != nil {
		return nil, err
	}
	out = append(out, matches...)

	archived := filepath.Join(root, "archived_sessions")
	_ = filepath.WalkDir(archived, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // archived_sessions absent: nothing to walk, not an error.
		}
		if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
			out = append(out, path)
		}
		return nil
	})
	return out, nil
}
