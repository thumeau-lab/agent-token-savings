package cliapp

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/ttlestimate"
)

// AgentTTLResult is one agent's entry in `ats ttl`'s stable JSON schema.
type AgentTTLResult struct {
	Agent              string  `json:"agent"`
	Skipped            bool    `json:"skipped"`
	Reason             string  `json:"reason,omitempty"`
	Days               int     `json:"days,omitempty"`
	MachineSessions    int64   `json:"machine_sessions,omitempty"`
	GapSessions        int64   `json:"gap_sessions,omitempty"`
	MachineWriteTokens int64   `json:"machine_write_tokens,omitempty"`
	TotalWriteTokens   int64   `json:"total_write_tokens,omitempty"`
	NoGapWriteTokens   int64   `json:"no_gap_write_tokens,omitempty"`
	EstimatedSaving    float64 `json:"estimated_saving_ratio,omitempty"`
	Recommendation     string  `json:"recommendation,omitempty"`

	// CurrentValue is the agent's current setting for the lever this
	// command inspects (currently: Hermes' prompt_caching.cache_ttl,
	// read via fixer.CurrentCacheTTL -- the same reader `ats fix` uses,
	// so `ats ttl` and `ats fix` never disagree about the current
	// value). Empty when the value could not be read.
	CurrentValue string `json:"current_value,omitempty"`

	// Headline is a single, self-contained plain-language sentence that
	// already states the magnitude AND the recommendation consistently
	// with each other (see ttlCard/renderTTLText): callers should prefer
	// it as the card verdict over gluing EstimatedSaving and
	// Recommendation together generically, which is what produced the
	// "saves 18.7%" / "no session goes idle" contradiction this field
	// exists to prevent.
	Headline string `json:"headline,omitempty"`
	// MethodNote explains, when non-empty, that tools/ttl_auto_estimate.py's
	// machine-vs-human session split does not apply to this agent and
	// what is computed instead (Claude Code only: every local session is
	// interactive, there is no machine session type to isolate).
	MethodNote string `json:"method_note,omitempty"`

	// Claude Code only: the direct per-call gap bucketing (see
	// claudecode.TTLResult) that MachineSessions/EstimatedSaving above
	// are derived from, exposed so the numbers behind the recommendation
	// are inspectable instead of opaque.
	Calls                int64 `json:"calls,omitempty"`
	FirstCallWriteTokens int64 `json:"first_call_write_tokens,omitempty"`
	MidGapWriteTokens    int64 `json:"mid_gap_write_tokens,omitempty"`
	MidGapCalls          int64 `json:"mid_gap_calls,omitempty"`
	LongGapWriteTokens   int64 `json:"long_gap_write_tokens,omitempty"`
	LongGapCalls         int64 `json:"long_gap_calls,omitempty"`
}

// TTLPayload is the top-level JSON object printed by `ats ttl --json`.
type TTLPayload struct {
	Command string           `json:"command"`
	Days    int              `json:"days"`
	Agents  []AgentTTLResult `json:"agents"`
}

func ttlSkipped(k agents.Kind, reason string) AgentTTLResult {
	return AgentTTLResult{Agent: string(k), Skipped: true, Reason: reason}
}

const ttlNotApplicableKey = "skip.ttl_not_applicable"

// claudeCodeTTLResult builds the AgentTTLResult for claude-code from the
// per-call gap cost model (claudecode.TTLResult): a single headline whose
// wording always matches the numbers next to it, in whichever direction
// the direct 5m-vs-1h cost comparison points.
func claudeCodeTTLResult(k agents.Kind, days int, res claudecode.TTLResult, lang i18n.Lang) AgentTTLResult {
	saving := res.SavingRatio()
	var rec, headline string
	if saving > 0 {
		savingPct := render.FormatPercentLang(saving*100, lang, 1)
		rec = i18n.T(lang, "ttl.cc_rec_switch", res.MidGapCalls, res.Calls)
		headline = i18n.T(lang, "ttl.cc_headline_switch", savingPct, rec)
	} else {
		extraPct := render.FormatPercentLang(-saving*100, lang, 1)
		rec = i18n.T(lang, "ttl.cc_rec_keep")
		headline = i18n.T(lang, "ttl.cc_headline_keep", extraPct, rec)
	}
	return AgentTTLResult{
		Agent: string(k), Days: days,
		MachineSessions: res.Sessions, GapSessions: res.MidGapCalls + res.LongGapCalls,
		MachineWriteTokens: res.TotalWriteTokens(), TotalWriteTokens: res.TotalWriteTokens(),
		EstimatedSaving: maxFloat(saving, 0), Recommendation: rec, Headline: headline,
		MethodNote: i18n.T(lang, "note.claude_method"),
		Calls:      res.Calls, FirstCallWriteTokens: res.FirstCallWriteTokens,
		MidGapWriteTokens: res.MidGapWriteTokens, MidGapCalls: res.MidGapCalls,
		LongGapWriteTokens: res.LongGapWriteTokens, LongGapCalls: res.LongGapCalls,
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func collectTTL(roots agents.Roots, agentFilter string, days int, lang i18n.Lang, now time.Time) []AgentTTLResult {
	var out []AgentTTLResult
	for _, k := range agents.All {
		if agentFilter != "" && string(k) != agentFilter {
			continue
		}
		switch k {
		case agents.Hermes:
			if !roots.HermesHasData() {
				out = append(out, ttlSkipped(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			res, err := ttlestimate.Estimate(roots.HermesHome, days)
			if err != nil {
				out = append(out, ttlSkipped(k, err.Error()))
				continue
			}
			configPath := filepath.Join(roots.HermesHome, "config.yaml")
			// Read the current value through the same reader `ats fix`
			// uses (fixer.DetectHermesCacheTTLFinding), so `ats ttl`
			// never recommends a change `ats fix` already considers
			// done. Errors (e.g. no config.yaml) are not fatal here:
			// the saving estimate above still stands on its own.
			currentTTL, _, _ := fixer.CurrentCacheTTL(configPath)
			saving := res.EstimatedSavingRatio()
			savingPct := render.FormatPercentLang(saving*100, lang, 1)
			var rec, headline string
			switch {
			case currentTTL == fixer.RecommendedCacheTTL:
				rec = ""
				headline = i18n.T(lang, "ttl.hermes_auto_on", savingPct)
			case res.NoGapWriteTokens() > 0:
				rec = i18n.T(lang, "ttl.hermes_rec_set_auto")
				headline = i18n.T(lang, "ttl.hermes_headline_switch", savingPct, rec)
			default:
				rec = i18n.T(lang, "ttl.hermes_rec_keep_1h")
				headline = i18n.T(lang, "ttl.hermes_headline_switch", savingPct, rec)
			}
			out = append(out, AgentTTLResult{
				Agent: string(k), Days: days,
				MachineSessions: res.MachineSessions, GapSessions: res.GapSessions,
				MachineWriteTokens: res.MachineWriteTokens, TotalWriteTokens: res.TotalWriteTokens,
				NoGapWriteTokens: res.NoGapWriteTokens(), EstimatedSaving: saving,
				Recommendation: rec, CurrentValue: currentTTL,
				Headline: headline,
			})
		case agents.ClaudeCode:
			if !roots.ClaudeHasData() {
				out = append(out, ttlSkipped(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			res, err := claudecode.EstimateTTL(roots.ClaudeHome, days, now)
			if err != nil {
				out = append(out, ttlSkipped(k, err.Error()))
				continue
			}
			out = append(out, claudeCodeTTLResult(k, days, res, lang))
		case agents.Codex, agents.Copilot, agents.GeminiCLI:
			out = append(out, ttlSkipped(k, i18n.T(lang, ttlNotApplicableKey)))
		}
	}
	return out
}

func runTTL(args []string, stdout, stderr io.Writer, env Env) int {
	lang := resolveLang(env)
	fs := flag.NewFlagSet("ttl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	days := fs.Int("days", 7, "size of the look-back window in days")
	jsonOut := fs.Bool("json", false, "print machine-readable JSON instead of styled text")
	agentFilter, ok := parsePositionalAndFlags(fs, args, lang)
	if !ok {
		return 2
	}

	roots, err := agents.DefaultRoots(env.HomeDir)
	if err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	now := resolveNow(env)

	// --json always stays English/machine format regardless of the UI
	// language: collect with i18n.EN so Reason/Headline/Recommendation/
	// MethodNote never leak a translated string into the payload.
	collectLang := lang
	if *jsonOut {
		collectLang = i18n.EN
	}
	results := collectTTL(roots, agentFilter, *days, collectLang, now)

	if *jsonOut {
		payload := TTLPayload{Command: "ttl", Days: *days, Agents: results}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintln(stderr, "ats:", err)
			return 1
		}
		return 0
	}

	renderTTLText(stdout, env, results)
	return 0
}
