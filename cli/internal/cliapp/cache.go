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
	"github.com/thumeau-lab/agent-token-savings/cli/internal/codexcli"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/copilotcli"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// WarmRebuildDetail is one abnormal cache rebuild shown in the "worst
// cache breaks" table: a warm session that suddenly rewrote the whole
// prefix instead of reading it from cache.
type WarmRebuildDetail struct {
	Session   string `json:"session"`
	Timestamp string `json:"timestamp"`
	Tokens    int64  `json:"tokens"`
}

// AgentCacheResult is one agent's entry in `ats cache`'s stable JSON schema.
type AgentCacheResult struct {
	Agent              string              `json:"agent"`
	Skipped            bool                `json:"skipped"`
	Reason             string              `json:"reason,omitempty"`
	Days               int                 `json:"days,omitempty"`
	Sessions           int64               `json:"sessions,omitempty"`
	Calls              int64               `json:"calls,omitempty"`
	CacheWriteTokens   int64               `json:"cache_write_tokens,omitempty"`
	CacheReadTokens    int64               `json:"cache_read_tokens,omitempty"`
	InputTokens        int64               `json:"input_tokens,omitempty"`
	OutputTokens       int64               `json:"output_tokens,omitempty"`
	HitRate            float64             `json:"hit_rate,omitempty"`
	WarmRebuilds       int                 `json:"warm_rebuilds,omitempty"`
	WarmRebuildDetails []WarmRebuildDetail `json:"warm_rebuild_details,omitempty"`
	USD                map[string]float64  `json:"usd,omitempty"`
	USDShare           map[string]float64  `json:"usd_share,omitempty"`
	// USDNote explains, when non-empty, that USD/USDShare is an
	// API-equivalent cost at list price, not a real per-token charge
	// (Claude Code on a subscription plan is not billed per token).
	USDNote string `json:"usd_note,omitempty"`
	// CostUnknown is set for agents with no pricing table entry for
	// their exact model (codex, copilot): no usd/usd_share is computed,
	// the card shows "unknown price"/"prix inconnu" instead (the card:
	// never invent a $ figure without a matching price table entry).
	CostUnknown bool `json:"cost_unknown,omitempty"`
	// CacheWriteUnknown is set when CacheWriteTokens cannot be trusted
	// (older codex-cli files lack cache_write_input_tokens entirely): the
	// card must show "n/a", never a bare 0.
	CacheWriteUnknown bool `json:"cache_write_unknown,omitempty"`
	// SessionsWithoutUsage counts sessions with no usable usage data
	// (codex: interactive sessions that never emitted a token_count
	// event, openai/codex#9660) -- counted here, never silently dropped.
	SessionsWithoutUsage int64 `json:"sessions_without_usage,omitempty"`
	// ModelBreakdown groups usage per model (copilot: auto-routes
	// requests across models, and a model switch empties the prompt
	// cache, so one blended total would hide that).
	ModelBreakdown []ModelUsageJSON `json:"model_breakdown,omitempty"`
}

// ModelUsageJSON is one model's aggregated usage row in the cache card's
// per-model table (copilot).
type ModelUsageJSON struct {
	Model            string  `json:"model"`
	Calls            int64   `json:"calls"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	HitRate          float64 `json:"hit_rate"`
}

// CachePayload is the top-level JSON object printed by `ats cache --json`.
type CachePayload struct {
	Command string             `json:"command"`
	Days    int                `json:"days"`
	Agents  []AgentCacheResult `json:"agents"`
}

func skippedResult(k agents.Kind, reason string) AgentCacheResult {
	return AgentCacheResult{Agent: string(k), Skipped: true, Reason: reason}
}

// usdSplit prices (input, output, cache_read, cache_write) token counts
// with the same Opus 5.5 table used for Hermes (hermesusage.PriceUSDPerMTok,
// docs/measurements.md), and returns both the per-category USD cost and
// its share of the total -- the only price table in this repo, so it is
// reused for claude-code instead of inventing a second one.
func usdSplit(input, output, cacheRead, cacheWrite int64) (usd, share map[string]float64) {
	tok := map[string]int64{
		"input": input, "output": output, "cache_read": cacheRead, "cache_write": cacheWrite,
	}
	usd = map[string]float64{}
	share = map[string]float64{}
	var total float64
	for k, v := range tok {
		u := float64(v) * hermesusage.PriceUSDPerMTok[k] / 1e6
		usd[k] = u
		total += u
	}
	for k, v := range usd {
		if total != 0 {
			share[k] = v / total
		}
	}
	return usd, share
}

// totalCacheUSD sums a.USD's four buckets (input/output/cache_read/
// cache_write) into the API-equivalent total cost for the window. 0 for
// agents with no pricing table entry (a.USD left empty, see CostUnknown).
func totalCacheUSD(a AgentCacheResult) float64 {
	return a.USD["input"] + a.USD["output"] + a.USD["cache_read"] + a.USD["cache_write"]
}

// costWithoutCache recomputes the window's cost treating every
// cache-read token as a plain uncached input token at the same
// per-token input price (hermesusage.PriceUSDPerMTok["input"]): "what
// this would have cost without the cache helping at all", for the
// Summary tab's money-amount saving (card: "sans le cache, ca aurait
// coute X $"). input/output/cache_write costs are taken from a.USD
// unchanged -- only the cache-read bucket is repriced.
func costWithoutCache(a AgentCacheResult) float64 {
	readAsInput := float64(a.CacheReadTokens) * hermesusage.PriceUSDPerMTok["input"] / 1e6
	return a.USD["input"] + a.USD["output"] + a.USD["cache_write"] + readAsInput
}

func warmRebuildDetails(rebuilds []claudecode.WarmRebuild) []WarmRebuildDetail {
	out := make([]WarmRebuildDetail, 0, len(rebuilds))
	for _, w := range rebuilds {
		out = append(out, WarmRebuildDetail{Session: w.Session, Timestamp: w.Timestamp, Tokens: w.Create})
	}
	return out
}

// claudeModelBreakdown converts claudecode.Report's per-model totals
// into the cache card's ModelUsageJSON rows (same shape copilot's
// per-model table already uses -- see copilotCacheResult): claude-code
// already has a pricing table entry (usdSplit), so unlike copilot this
// is only token counts + hit rate, not a cost figure, matching the
// panel's existing columns (Model, HitRate, Calls -- see
// renderPanelByModel/dashboardModelRow).
func claudeModelBreakdown(models []claudecode.ModelUsage) []ModelUsageJSON {
	out := make([]ModelUsageJSON, 0, len(models))
	for _, m := range models {
		out = append(out, ModelUsageJSON{
			Model: m.Model, Calls: int64(m.Calls),
			InputTokens: m.Totals.Input, OutputTokens: m.Totals.Output,
			CacheReadTokens: m.Totals.Read, CacheWriteTokens: m.Totals.Create,
			HitRate: m.Totals.HitRate(),
		})
	}
	return out
}

// hermesModelBreakdown converts hermesusage.QueryByModel's per-model
// totals (the sessions table's own `model` column) into the cache
// card's ModelUsageJSON rows, same shape every other agent's
// per-model table uses.
func hermesModelBreakdown(models []hermesusage.ModelUsage) []ModelUsageJSON {
	out := make([]ModelUsageJSON, 0, len(models))
	for _, m := range models {
		out = append(out, ModelUsageJSON{
			Model: m.Model, Calls: m.Calls,
			InputTokens: m.Input, OutputTokens: m.Output,
			CacheReadTokens: m.CacheRead, CacheWriteTokens: m.CacheWrite,
			HitRate: m.HitRate(),
		})
	}
	return out
}

// codexModelBreakdown converts codexcli.Report's per-model totals
// (grouped from each rollout file's "turn_context" lines) into the
// cache card's ModelUsageJSON rows, same shape every other agent's
// per-model table uses.
func codexModelBreakdown(models []codexcli.ModelUsage) []ModelUsageJSON {
	out := make([]ModelUsageJSON, 0, len(models))
	for _, m := range models {
		out = append(out, ModelUsageJSON{
			Model: m.Model, Calls: int64(m.Calls),
			InputTokens: m.Totals.Uncached(), OutputTokens: m.Totals.Output,
			CacheReadTokens: m.Totals.CachedInput, CacheWriteTokens: m.Totals.CacheWrite,
			HitRate: m.Totals.HitRate(),
		})
	}
	return out
}

// codexCacheResult builds the AgentCacheResult for codex from
// codexcli.Aggregate's Report: usd/usd_share are computed via
// openAICostForModels when the window's whole usage is attributable to
// one or more models in openAIPriceUSDPerMTok, otherwise CostUnknown is
// set (the card: show tokens and hit rate only, never invent a $
// figure for an unpriced or unknown model).
func codexCacheResult(k agents.Kind, days int, rep codexcli.Report) AgentCacheResult {
	breakdown := codexModelBreakdown(rep.ModelBreakdown)
	usd, share, costUnknown := openAICostForModels(
		rep.Totals.Uncached(), rep.Totals.Output, rep.Totals.CachedInput, rep.Totals.CacheWrite, breakdown)
	return AgentCacheResult{
		Agent: string(k), Days: days,
		Sessions: int64(rep.Sessions), Calls: int64(rep.Calls),
		CacheWriteTokens: rep.Totals.CacheWrite, CacheWriteUnknown: rep.Totals.CacheWriteUnknown,
		CacheReadTokens: rep.Totals.CachedInput,
		InputTokens:     rep.Totals.Uncached(), OutputTokens: rep.Totals.Output,
		HitRate:              rep.Totals.HitRate(),
		SessionsWithoutUsage: int64(rep.SessionsWithoutUsage),
		USD:                  usd, USDShare: share, CostUnknown: costUnknown,
		ModelBreakdown: breakdown,
	}
}

// copilotCacheResult builds the AgentCacheResult for copilot from
// copilotcli.QueryByModel's per-model rows: totals are the sum across
// models (for the top-level stat grid), and ModelBreakdown carries the
// per-model detail (a model switch empties the prompt cache, so one
// blended total alone would hide that -- see the card). usd/usd_share
// are computed via openAICostForModels when every model in the window
// is an OpenAI model in openAIPriceUSDPerMTok (e.g. gpt-5); otherwise
// CostUnknown is set. Copilot is billed by subscription, not per token,
// so a priced result also carries note.copilot_usd -- the same
// API-equivalent-cost caveat Claude Code's note.claude_usd carries.
func copilotCacheResult(k agents.Kind, days int, models []copilotcli.ModelUsage, lang i18n.Lang) AgentCacheResult {
	var sessions, calls, uncached, output, read, write int64
	breakdown := make([]ModelUsageJSON, 0, len(models))
	for _, m := range models {
		sessions += m.Sessions
		calls += m.Calls
		uncached += m.Uncached()
		output += m.OutputTokens
		read += m.CacheReadTokens
		write += m.CacheWriteTokens
		breakdown = append(breakdown, ModelUsageJSON{
			Model: m.Model, Calls: m.Calls,
			InputTokens: m.Uncached(), OutputTokens: m.OutputTokens,
			CacheReadTokens: m.CacheReadTokens, CacheWriteTokens: m.CacheWriteTokens,
			HitRate: m.HitRate(),
		})
	}
	var hitRate float64
	if denom := uncached + read + write; denom > 0 {
		hitRate = float64(read) / float64(denom)
	}
	usd, share, costUnknown := openAICostForModels(uncached, output, read, write, breakdown)
	usdNote := ""
	if !costUnknown {
		usdNote = i18n.T(lang, "note.copilot_usd")
	}
	return AgentCacheResult{
		Agent: string(k), Days: days,
		Sessions: sessions, Calls: calls,
		CacheWriteTokens: write, CacheReadTokens: read,
		InputTokens: uncached, OutputTokens: output,
		HitRate: hitRate, CostUnknown: costUnknown,
		USD: usd, USDShare: share, USDNote: usdNote,
		ModelBreakdown: breakdown,
	}
}

// codexSkipReason (i18n key "skip.codex") explains exactly why `ats
// cache` still skips Codex even when ~/.codex is present: Codex
// interactive sessions do not reliably emit `token_count` events to
// their session JSONL files upstream (openai/codex#9660, "Include
// token_count events in non-interactive session files"), so there is no
// stable schema to parse yet -- not a generic "not parsed yet"
// placeholder. geminiSkipReason is "skip.gemini".
const (
	codexSkipKey  = "skip.codex"
	geminiSkipKey = "skip.gemini"
)

func collectCache(roots agents.Roots, agentFilter string, days int, lang i18n.Lang, now time.Time) []AgentCacheResult {
	var out []AgentCacheResult
	for _, k := range agents.All {
		if agentFilter != "" && string(k) != agentFilter {
			continue
		}
		switch k {
		case agents.ClaudeCode:
			if !roots.ClaudeHasData() {
				out = append(out, skippedResult(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			rep, err := claudecode.Aggregate(roots.ClaudeHome, days, now)
			if err != nil {
				out = append(out, skippedResult(k, err.Error()))
				continue
			}
			warm := len(rep.WarmRebuilds)
			usd, share := usdSplit(rep.Totals.Input, rep.Totals.Output, rep.Totals.Read, rep.Totals.Create)
			out = append(out, AgentCacheResult{
				Agent: string(k), Days: days, Sessions: int64(rep.Sessions), Calls: int64(rep.Calls),
				CacheWriteTokens: rep.Totals.Create, CacheReadTokens: rep.Totals.Read,
				InputTokens: rep.Totals.Input, OutputTokens: rep.Totals.Output,
				HitRate: rep.Totals.HitRate(), WarmRebuilds: warm,
				WarmRebuildDetails: warmRebuildDetails(rep.WarmRebuilds),
				USD:                usd, USDShare: share, USDNote: i18n.T(lang, "note.claude_usd"),
				ModelBreakdown: claudeModelBreakdown(rep.ModelBreakdown),
			})
		case agents.Hermes:
			if !roots.HermesHasData() {
				out = append(out, skippedResult(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			rows, err := hermesusage.QueryDaily(filepath.Join(roots.HermesHome, "state.db"), days)
			if err != nil {
				out = append(out, skippedResult(k, err.Error()))
				continue
			}
			s := hermesusage.Summarize(rows)
			// QueryByModel reads the sessions table's own `model`
			// column (card: "Hermes state.db sessions carry the
			// model"): its own error is never fatal to the agent's
			// whole result -- an older/custom schema without that
			// column just means no per-model breakdown, same as any
			// other agent with ModelBreakdown left empty.
			models, _ := hermesusage.QueryByModel(filepath.Join(roots.HermesHome, "state.db"), days)
			out = append(out, AgentCacheResult{
				Agent: string(k), Days: days, Sessions: s.Sessions, Calls: s.Calls,
				CacheWriteTokens: s.CacheWrite, CacheReadTokens: s.CacheRead,
				InputTokens: s.Input, OutputTokens: s.Output,
				HitRate: s.HitRate, USD: s.USD, USDShare: s.USDShare,
				ModelBreakdown: hermesModelBreakdown(models),
			})
		case agents.Codex:
			if !roots.CodexHasData() {
				out = append(out, skippedResult(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			if !roots.CodexSessionFilesExist() {
				out = append(out, skippedResult(k, i18n.T(lang, codexSkipKey)))
				continue
			}
			rep, err := codexcli.Aggregate(roots.CodexHome)
			if err != nil {
				out = append(out, skippedResult(k, err.Error()))
				continue
			}
			out = append(out, codexCacheResult(k, days, rep))
		case agents.Copilot:
			if !roots.CopilotHasData() {
				out = append(out, skippedResult(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			models, err := copilotcli.QueryByModel(filepath.Join(roots.CopilotHome, "session-store.db"), days, now)
			if err != nil {
				out = append(out, skippedResult(k, err.Error()))
				continue
			}
			if len(models) == 0 {
				out = append(out, skippedResult(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			out = append(out, copilotCacheResult(k, days, models, lang))
		case agents.GeminiCLI:
			if !roots.GeminiHasData() {
				out = append(out, skippedResult(k, i18n.T(lang, "skip.no_local_data")))
				continue
			}
			out = append(out, skippedResult(k, i18n.T(lang, geminiSkipKey)))
		}
	}
	return out
}

func runCache(args []string, stdout, stderr io.Writer, env Env) int {
	lang := resolveLang(env)
	fs := flag.NewFlagSet("cache", flag.ContinueOnError)
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

	// --json always stays English/machine format regardless of the UI
	// language: collect with i18n.EN so Reason/USDNote never leak a
	// translated string into the machine-readable payload.
	collectLang := lang
	if *jsonOut {
		collectLang = i18n.EN
	}
	results := collectCache(roots, agentFilter, *days, collectLang, resolveNow(env))

	if *jsonOut {
		payload := CachePayload{Command: "cache", Days: *days, Agents: results}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintln(stderr, "ats:", err)
			return 1
		}
		return 0
	}

	renderCacheText(stdout, env, results)
	return 0
}
