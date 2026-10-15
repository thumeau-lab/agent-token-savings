package cliapp

import (
	"fmt"
	"io"
	"strconv"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/prefixinfo"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// cardWidth picks the card width for colored output: the detected
// terminal width when known, clamped to a sane range, else 80.
func cardWidth(env Env) int {
	w := env.TermWidth
	if w < 40 {
		w = 80
	}
	if w > 160 {
		w = 160
	}
	return w
}

// printSkipLine prints one skipped-agent line. On a TTY it is a single
// short muted note ("<agent>: no usable local data") instead of the raw
// Reason text, which used to read as a long line of clutter between
// cards (review feedback); the full reason stays available in --json and,
// in the TUI, as a dim note under the agent selector. Off a TTY (piped,
// redirected) the full reason is kept inline, unchanged from before.
func printSkipLine(stdout io.Writer, r render.Renderer, isTTY bool, lang i18n.Lang, agent, reason string) {
	if !isTTY {
		fmt.Fprintln(stdout, i18n.T(lang, "text.skip_piped", agent, reason))
		return
	}
	fmt.Fprintln(stdout, r.Muted(i18n.T(lang, "text.skip_tty", agent)))
}

func renderCacheText(stdout io.Writer, env Env, results []AgentCacheResult) {
	lang := resolveLang(env)
	r := render.New(render.Options{NoColor: env.NoColor, IsTTY: env.IsTTY, Dark: env.Dark, ForceASCII: env.ForceASCII})
	for _, a := range results {
		if a.Skipped {
			printSkipLine(stdout, r, env.IsTTY, lang, a.Agent, a.Reason)
			continue
		}
		if r.Colored() {
			fmt.Fprintln(stdout, r.RenderCard(cacheCard(a, lang), cardWidth(env)))
			continue
		}
		fmt.Fprintln(stdout, i18n.T(lang, "text.cache_summary", a.Agent, a.Sessions, a.Calls, a.Days))
		if a.CacheWriteUnknown {
			fmt.Fprintln(stdout, i18n.T(lang, "text.cache_tokens_write_na", a.CacheReadTokens, a.InputTokens, a.OutputTokens))
		} else {
			fmt.Fprintln(stdout, i18n.T(lang, "text.cache_tokens", a.CacheWriteTokens, a.CacheReadTokens, a.InputTokens, a.OutputTokens))
		}
		fmt.Fprintln(stdout, i18n.T(lang, "text.hit_rate_line", r.Bar(a.HitRate, 30), render.FormatPercentLang(a.HitRate*100, lang, 1)))
		if a.SessionsWithoutUsage > 0 {
			fmt.Fprintln(stdout, i18n.T(lang, "text.sessions_without_usage", a.SessionsWithoutUsage))
		}
		if a.WarmRebuilds > 0 {
			fmt.Fprintln(stdout, i18n.T(lang, "text.warm_rebuilds", a.WarmRebuilds))
			for _, d := range a.WarmRebuildDetails {
				fmt.Fprintln(stdout, i18n.T(lang, "text.warm_detail", d.Session, d.Timestamp, d.Tokens))
			}
		}
		if a.CostUnknown {
			fmt.Fprintln(stdout, i18n.T(lang, "text.cost_unknown"))
		}
		for _, mu := range a.ModelBreakdown {
			fmt.Fprintln(stdout, i18n.T(lang, "text.model_breakdown_row",
				mu.Model, mu.InputTokens, mu.OutputTokens, mu.CacheReadTokens, mu.CacheWriteTokens,
				render.FormatPercentLang(mu.HitRate*100, lang, 1)))
		}
		if len(a.USDShare) > 0 {
			if a.USDNote != "" {
				fmt.Fprintln(stdout, i18n.T(lang, "text.usd_note", a.USDNote))
			}
			for _, k := range []string{"cache_write", "cache_read", "input", "output"} {
				fmt.Fprintln(stdout, i18n.T(lang, "text.usd_share_line", i18n.T(lang, cacheVerdictBarLabel[k]), r.Bar(a.USDShare[k], 30), render.FormatPercentLang(a.USDShare[k]*100, lang, 0)))
			}
		}
	}
}

// cacheVerdictBarLabel maps a USD cost category to its i18n catalog key
// for the short label shown next to its bar in the card.
var cacheVerdictBarLabel = map[string]string{
	"cache_write": "bar.write_usd",
	"cache_read":  "bar.read_usd",
	"input":       "bar.input_usd",
	"output":      "bar.output_usd",
}

// cacheCard builds the colored Card for one `ats cache` agent result: a
// one-line verdict derived from the numbers (API-equivalent cost share +
// warm rebuilds, no invented advice), a 2-column stat grid of humanized
// numbers, a hit rate bar (colored by HitRateSemantic) plus one bar per
// cost-share category (colored by CostBucketColor, with a legend), and
// the worst cache breaks table when there are any.
func cacheCard(a AgentCacheResult, lang i18n.Lang) render.Card {
	r := render.New(render.Options{Dark: true}) // color-only helpers (SemanticColor/CostBucketColor); Dark is irrelevant to RGB equality, only used for the actual RenderCard call's own renderer
	totalUSD := totalCacheUSD(a)
	cacheWriteValue := render.FormatCountLang(a.CacheWriteTokens, lang)
	if a.CacheWriteUnknown {
		cacheWriteValue = i18n.T(lang, "text.not_applicable")
	}
	stats := []render.Stat{
		{Label: i18n.T(lang, "stat.sessions"), Value: render.FormatCountLang(a.Sessions, lang)},
		{Label: i18n.T(lang, "stat.calls"), Value: render.FormatCountLang(a.Calls, lang)},
		{Label: i18n.T(lang, "stat.cache_write"), Value: cacheWriteValue},
		{Label: i18n.T(lang, "stat.cache_read"), Value: render.FormatCountLang(a.CacheReadTokens, lang)},
		{Label: i18n.T(lang, "stat.input"), Value: render.FormatCountLang(a.InputTokens, lang)},
		{Label: i18n.T(lang, "stat.output"), Value: render.FormatCountLang(a.OutputTokens, lang)},
		{Label: i18n.T(lang, "stat.hit_rate"), Value: render.FormatPercentLang(a.HitRate*100, lang, 1)},
	}
	if a.SessionsWithoutUsage > 0 {
		stats = append(stats, render.Stat{Label: i18n.T(lang, "stat.sessions_without_usage"), Value: render.FormatCountLang(a.SessionsWithoutUsage, lang)})
	}
	if len(a.USD) > 0 {
		// "API-equivalent cost", not "Bill": Claude Code on a subscription
		// plan is not billed per token (see a.USDNote for the full caveat,
		// carried separately in the JSON so the label itself stays short).
		stats = append(stats, render.Stat{Label: i18n.T(lang, "stat.api_equiv_cost"), Value: render.FormatUSDLang(totalUSD, lang)})
	} else if a.CostUnknown {
		stats = append(stats, render.Stat{Label: i18n.T(lang, "stat.api_equiv_cost"), Value: i18n.T(lang, "text.unknown_price")})
	}

	bars := []render.BarStat{
		{Label: i18n.T(lang, "bar.hit_rate"), Frac: a.HitRate, Annotation: render.FormatPercentLang(a.HitRate*100, lang, 1), Color: r.SemanticColor(render.HitRateSemantic(a.HitRate))},
	}
	var legend []render.LegendItem
	if len(a.USDShare) > 0 {
		for _, k := range []string{"cache_write", "cache_read", "input", "output"} {
			color := r.CostBucketColor(k)
			bars = append(bars, render.BarStat{
				Label: i18n.T(lang, cacheVerdictBarLabel[k]), Frac: a.USDShare[k],
				Annotation: render.FormatPercentLang(a.USDShare[k]*100, lang, 0), Color: color,
			})
			legend = append(legend, render.LegendItem{Label: i18n.T(lang, cacheVerdictBarLabel[k]), Color: color})
		}
	}

	var tbl *render.CardTable
	switch {
	case len(a.WarmRebuildDetails) > 0:
		rows := make([][]string, 0, len(a.WarmRebuildDetails))
		for _, d := range a.WarmRebuildDetails {
			rows = append(rows, []string{d.Session, d.Timestamp, render.FormatCountLang(d.Tokens, lang)})
		}
		tbl = &render.CardTable{
			Title: i18n.T(lang, "table.worst_cache_breaks"),
			Headers: []string{
				i18n.T(lang, "table.header_session"), i18n.T(lang, "table.header_time"), i18n.T(lang, "table.header_tokens_rewritten"),
			},
			Rows: rows,
		}
	case len(a.ModelBreakdown) > 0:
		rows := make([][]string, 0, len(a.ModelBreakdown))
		for _, mu := range a.ModelBreakdown {
			rows = append(rows, []string{
				mu.Model, render.FormatCountLang(mu.InputTokens, lang), render.FormatCountLang(mu.OutputTokens, lang),
				render.FormatCountLang(mu.CacheReadTokens, lang), render.FormatCountLang(mu.CacheWriteTokens, lang),
				render.FormatPercentLang(mu.HitRate*100, lang, 1),
			})
		}
		tbl = &render.CardTable{
			Title: i18n.T(lang, "table.usage_by_model"),
			Headers: []string{
				i18n.T(lang, "table.header_model"), i18n.T(lang, "table.header_input"), i18n.T(lang, "table.header_output"),
				i18n.T(lang, "table.header_read"), i18n.T(lang, "table.header_write"), i18n.T(lang, "bar.hit_rate"),
			},
			Rows: rows,
		}
	}

	return render.Card{
		Key: "1", Title: a.Agent, Subtitle: i18n.T(lang, "card.last_days", a.Days),
		Verdict: cacheVerdict(a, lang), Stats: stats, Bars: bars, Legend: legend, Table: tbl,
	}
}

// cacheVerdict renders the card's one-line plain-words verdict, derived
// only from the numbers already computed (no invented advice).
func cacheVerdict(a AgentCacheResult, lang i18n.Lang) string {
	if len(a.USDShare) > 0 {
		writeShare := render.FormatPercentLang(a.USDShare["cache_write"]*100, lang, 0)
		if a.WarmRebuilds > 0 {
			if a.WarmRebuilds == 1 {
				return i18n.T(lang, "verdict.cache_write_rebuilt_one", writeShare, a.WarmRebuilds)
			}
			return i18n.T(lang, "verdict.cache_write_rebuilt_many", writeShare, a.WarmRebuilds)
		}
		return i18n.T(lang, "verdict.cache_write_no_rebuild", writeShare, a.Sessions)
	}
	verdict := i18n.T(lang, "verdict.hit_rate_only", render.FormatPercentLang(a.HitRate*100, lang, 0), a.Calls)
	if len(a.ModelBreakdown) > 1 {
		verdict += " " + i18n.T(lang, "note.copilot_model_switch")
	}
	return verdict
}

func renderPrefixText(stdout io.Writer, env Env, results []AgentPrefixResult) {
	lang := resolveLang(env)
	r := render.New(render.Options{NoColor: env.NoColor, IsTTY: env.IsTTY, Dark: env.Dark, ForceASCII: env.ForceASCII})
	for _, a := range results {
		if a.Skipped {
			printSkipLine(stdout, r, env.IsTTY, lang, a.Agent, a.Reason)
			continue
		}
		if r.Colored() {
			fmt.Fprintln(stdout, r.RenderCard(prefixCard(a, lang), cardWidth(env)))
			continue
		}
		fmt.Fprintln(stdout, i18n.T(lang, "text.prefix_summary", a.Agent, a.TotalBytes, a.EstimatedTokens, a.TokenEstimate))
		for _, c := range a.Components {
			var frac float64
			if a.TotalBytes > 0 {
				frac = float64(c.Bytes) / float64(a.TotalBytes)
			}
			fmt.Fprintln(stdout, i18n.T(lang, "text.prefix_component", c.Name, r.Bar(frac, 20), c.Bytes))
		}
		if a.BuiltinTokens > 0 {
			fmt.Fprintln(stdout, i18n.T(lang, "text.prefix_builtin", "", a.BuiltinTokens, a.BuiltinSource))
		}
		if a.MCPServersNote != "" {
			fmt.Fprintln(stdout, i18n.T(lang, "text.prefix_mcp_note", a.MCPServersNote))
		}
	}
}

// prefixCard builds the colored Card for one `ats prefix` agent result:
// a verdict naming the dominant locally-measured component (or the
// built-in figure when that dwarfs everything local), the component
// breakdown as bars (one per component, sized by share of TotalBytes),
// and the built-in line and MCP server count as their own stats since
// they have no byte share.
func prefixCard(a AgentPrefixResult, lang i18n.Lang) render.Card {
	stats := []render.Stat{
		{Label: i18n.T(lang, "stat.local_total"), Value: render.FormatBytes(a.TotalBytes)},
		{Label: i18n.T(lang, "stat.estimated_tok"), Value: render.FormatCountLang(a.EstimatedTokens, lang) + " tok"},
	}
	if a.BuiltinTokens > 0 {
		stats = append(stats, render.Stat{Label: i18n.T(lang, "stat.builtin_tok"), Value: render.FormatCountLang(a.BuiltinTokens, lang) + " tok"})
	}
	if a.MCPServersNote != "" {
		stats = append(stats, render.Stat{Label: i18n.T(lang, "stat.mcp_servers"), Value: render.FormatCountLang(int64(a.MCPServerCount), lang)})
	}

	var bars []render.BarStat
	for _, c := range a.Components {
		var frac float64
		if a.TotalBytes > 0 {
			frac = float64(c.Bytes) / float64(a.TotalBytes)
		}
		bars = append(bars, render.BarStat{
			Label: c.Name, Frac: frac, Annotation: render.FormatBytes(c.Bytes),
		})
	}

	verdict := prefixVerdict(a, lang)
	return render.Card{Key: "2", Title: a.Agent, Verdict: verdict, Stats: stats, Bars: bars}
}

// prefixVerdict names, in plain words, what dominates the prefix: the
// built-in (not locally measurable) figure when it is known and larger
// than the local total, otherwise the single biggest local component.
func prefixVerdict(a AgentPrefixResult, lang i18n.Lang) string {
	if a.BuiltinTokens > 0 && a.BuiltinTokens >= a.EstimatedTokens {
		return i18n.T(lang, "verdict.builtin_outweighs",
			render.FormatCountLang(a.BuiltinTokens, lang), render.FormatCountLang(a.EstimatedTokens, lang), builtinCitation(a, lang))
	}
	var biggest ComponentJSON
	for _, c := range a.Components {
		if c.Bytes > biggest.Bytes {
			biggest = c
		}
	}
	if biggest.Name == "" {
		return i18n.T(lang, "verdict.estimated_local_prefix", render.FormatCountLang(a.EstimatedTokens, lang))
	}
	share := 0.0
	if a.TotalBytes > 0 {
		share = float64(biggest.Bytes) / float64(a.TotalBytes) * 100
	}
	return i18n.T(lang, "verdict.component_share", biggest.Name, render.FormatPercentLang(share, lang, 0), render.FormatCountLang(a.EstimatedTokens, lang))
}

// builtinCitation returns the localized citation sentence for
// prefixVerdict's builtin_outweighs branch: Claude Code's measured
// default-vs-no-tools prefix numbers (prefixinfo.ClaudeCode*PrefixTokens),
// localized through the catalog, instead of embedding
// AgentPrefixResult.BuiltinSource's English prose directly into a
// French sentence (that field keeps citing bench/claude_code_levers.py
// in English for --json, which always stays English: see runPrefix's
// collectLang). Any other agent that ever sets BuiltinTokens falls back
// to BuiltinSource itself, since there is no known localized citation
// for it yet.
func builtinCitation(a AgentPrefixResult, lang i18n.Lang) string {
	if a.Agent != "claude-code" {
		return a.BuiltinSource
	}
	pct := float64(prefixinfo.ClaudeCodeDefaultPrefixTokens-prefixinfo.ClaudeCodeNoToolsPrefixTokens) / float64(prefixinfo.ClaudeCodeDefaultPrefixTokens) * 100
	return i18n.T(lang, "prefix.claude_builtin_citation",
		render.FormatCountLang(prefixinfo.ClaudeCodeDefaultPrefixTokens, lang),
		render.FormatCountLang(prefixinfo.ClaudeCodeNoToolsPrefixTokens, lang),
		strconv.FormatFloat(pct, 'f', 0, 64))
}

func renderTTLText(stdout io.Writer, env Env, results []AgentTTLResult) {
	lang := resolveLang(env)
	r := render.New(render.Options{NoColor: env.NoColor, IsTTY: env.IsTTY, Dark: env.Dark, ForceASCII: env.ForceASCII})
	for _, a := range results {
		if a.Skipped {
			printSkipLine(stdout, r, env.IsTTY, lang, a.Agent, a.Reason)
			continue
		}
		if r.Colored() {
			fmt.Fprintln(stdout, r.RenderCard(ttlCard(a, lang), cardWidth(env)))
			continue
		}
		fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_summary", a.Agent, a.Days))
		if a.MethodNote != "" {
			fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_note", a.MethodNote))
			fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_cc_detail", a.MachineSessions, a.Calls, a.MidGapCalls, a.LongGapCalls))
		} else {
			fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_hermes_detail", a.MachineSessions, a.GapSessions))
			fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_write_share", a.MachineWriteTokens, a.TotalWriteTokens))
		}
		fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_headline", a.Headline))
		fmt.Fprintln(stdout, i18n.T(lang, "text.ttl_recommendation", a.Recommendation))
	}
}

// ttlCard builds the colored Card for one `ats ttl` agent result. The
// verdict is always a.Headline: a single sentence already crafted (in
// collectTTL) to state the magnitude and the recommendation coherently
// with each other, instead of gluing a generic "saves X%" template onto
// a per-agent recommendation that may point the other way (the exact bug
// that made claude-code's card self-contradictory). Claude Code gets its
// own stat set (sessions/calls/gap buckets from its own per-call cost
// model) since it has no machine-vs-human session split to report.
func ttlCard(a AgentTTLResult, lang i18n.Lang) render.Card {
	subtitle := i18n.T(lang, "card.last_days", a.Days)
	if a.MethodNote != "" { // claude-code: its own per-call gap buckets, not a machine/human split.
		stats := []render.Stat{
			{Label: i18n.T(lang, "stat.sessions"), Value: render.FormatCountLang(a.MachineSessions, lang)},
			{Label: i18n.T(lang, "stat.calls"), Value: render.FormatCountLang(a.Calls, lang)},
			{Label: i18n.T(lang, "stat.first_call_writes"), Value: render.FormatCountLang(a.FirstCallWriteTokens, lang)},
			{Label: i18n.T(lang, "stat.mid_gap_writes"), Value: render.FormatCountLang(a.MidGapWriteTokens, lang)},
			{Label: i18n.T(lang, "stat.long_gap_writes"), Value: render.FormatCountLang(a.LongGapWriteTokens, lang)},
		}
		return render.Card{Key: "3", Title: a.Agent, Subtitle: subtitle, Verdict: a.Headline, Stats: stats}
	}
	noGapShare := 0.0
	if a.TotalWriteTokens > 0 {
		noGapShare = float64(a.NoGapWriteTokens) / float64(a.TotalWriteTokens)
	}
	stats := []render.Stat{
		{Label: i18n.T(lang, "stat.sessions"), Value: render.FormatCountLang(a.MachineSessions, lang)},
		{Label: i18n.T(lang, "stat.gap_5min"), Value: render.FormatCountLang(a.GapSessions, lang)},
		{Label: i18n.T(lang, "stat.writes"), Value: render.FormatCountLang(a.MachineWriteTokens, lang)},
		{Label: i18n.T(lang, "stat.of_total"), Value: render.FormatCountLang(a.TotalWriteTokens, lang)},
	}
	bars := []render.BarStat{
		{Label: i18n.T(lang, "bar.no_gap_share"), Frac: noGapShare, Annotation: render.FormatPercentLang(noGapShare*100, lang, 0)},
	}
	return render.Card{Key: "3", Title: a.Agent, Subtitle: subtitle, Verdict: a.Headline, Stats: stats, Bars: bars}
}
