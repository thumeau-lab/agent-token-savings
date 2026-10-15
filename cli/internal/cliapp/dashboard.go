package cliapp

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// dashboardHeatmapLookbackDays is how far back the Activite heatmap's
// dedicated Hermes query reaches, regardless of the selected window
// (card: "read further back than the window for this panel only"):
// comfortably more than dashboardHeatmapWeeks*7 (84) days so the panel
// never runs short of history when `now` falls mid-week.
const dashboardHeatmapLookbackDays = 100

// dashboardFinding is one row in the "A ameliorer" panel: a short
// plain-words label, a pre-formatted $ or token estimate (empty when
// nothing was computed for it), the key that opens the matching detail
// tab, and Magnitude -- never displayed, only used by rankFindings to
// sort biggest-impact-first.
type dashboardFinding struct {
	Label     string
	Amount    string
	DetailKey string
	Magnitude float64
}

// rankFindings sorts findings by Magnitude, biggest first, stably (two
// findings with the same estimated impact keep their original relative
// order instead of shuffling between runs/platforms).
func rankFindings(findings []dashboardFinding) []dashboardFinding {
	out := make([]dashboardFinding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Magnitude > out[j].Magnitude
	})
	return out
}

// dashboardSaved is the "economise grace au cache" figure the card
// requires: cost-without-cache minus the real cost, using the exact
// same two cost functions `ats cache`/the Cache panel already compute
// (costWithoutCache, totalCacheUSD) -- never a second cost model. 0
// for an agent with no pricing table entry (a.USD left empty,
// CostUnknown=true), never an invented dollar figure.
func dashboardSaved(a AgentCacheResult) float64 {
	if len(a.USD) == 0 {
		return 0
	}
	return costWithoutCache(a) - totalCacheUSD(a)
}

// dashboardAgentRow is one row of the "Taux de cache par agent" panel.
type dashboardAgentRow struct {
	Agent     string
	HitRate   float64
	Calls     int64
	CostKnown bool
	Cost      float64
	Saved     float64
}

// dashboardDayRow is one row of the "Par jour" panel.
type dashboardDayRow struct {
	Day     string
	Cost    float64
	HitRate float64
}

// dashboardModelRow is one row of the "Par modele" panel. Agent is
// the model's owning agent (card follow-up point 1/B: group by agent
// so the same model name reported by two agents, e.g. claude-opus-5
// from both claude-code and hermes, never piles up unlabeled).
type dashboardModelRow struct {
	Agent   string
	Model   string
	HitRate float64
	Calls   int64
}

// dashboardMeasuredRow is one row of the "Mesure" panel: what ats
// actually read for this agent, and what it knowingly does not report.
type dashboardMeasuredRow struct {
	Agent           string
	Source          string
	Period          string
	Sessions        int64
	SessionsNoUsage int64
	Note            string
}

// dashboardData is everything the dashboard renderer needs, already
// computed from the same collectCache/collectTTL/fixer results every
// other tab uses -- no second computation path.
type dashboardData struct {
	ScopeAgent                             string // "" = every detected agent combined
	HeroCost                               float64
	HeroCostKnown                          bool
	HeroSaved                              float64
	HeroHitRate                            float64
	HeroCalls                              int64
	HeroSessions                           int64
	TokRead, TokWrite, TokInput, TokOutput int64
	CompareLine                            string // dim, "" when there is nothing worth showing

	// HeroCostExcluded lists, in first-seen order, the in-scope
	// agents left out of HeroCost/HeroSaved because their price is
	// unknown (CostUnknown): HeroCost only ever sums priced agents,
	// so this lets the renderer label the figure honestly (card
	// follow-up point C: "cout (hors codex, copilot)") instead of
	// implying it covers every detected agent. Empty when every
	// in-scope agent is priced.
	HeroCostExcluded []string

	// Hero mini-stats (card 1a): yesterday's cost, the 7-day average,
	// and the swing vs the day before yesterday, from the exact same
	// per-day series (DayRows) panel "Par jour" used to build from.
	// *Known is false when there is no daily history to compute from
	// (e.g. codex/copilot/gemini-cli alone) -- never an invented 0.
	MiniYesterday        float64
	MiniYesterdayKnown   bool
	MiniAvg7             float64
	MiniAvg7Known        bool
	MiniVsYesterdayPct   float64
	MiniVsYesterdayKnown bool

	AgentRows       []dashboardAgentRow
	AnyUnknownPrice bool

	DayRows     []dashboardDayRow
	NoDailyNote string

	Heatmap   dashboardHeatmap
	Histogram dashboardHistogram

	ModelRows []dashboardModelRow
	ModelNote string

	Findings []dashboardFinding

	Efficiency dashboardEfficiency

	Measured    []dashboardMeasuredRow
	IgnoredNote string
}

// dashboardHeatmapWeeks is the Activite panel's fixed column count
// (card: "min 12 weeks when data exists"): always 12 full calendar
// weeks ending on the week containing `now`, regardless of how much
// real history is behind it (a fresh install just shows mostly empty
// cells, never fewer columns).
const dashboardHeatmapWeeks = 12

// heatmapDay is one cell of the Activite heatmap.
type heatmapDay struct {
	Date    string // "YYYY-MM-DD"
	Cost    float64
	HasData bool
}

// dashboardHeatmap is panel "Activite"'s computed grid: Weeks[w][d],
// oldest week first, d=0 Monday .. d=6 Sunday (card: "7 rows
// (lun..dim)"). MonthStarts has one entry per week column, the
// calendar month (1-12) of that column's Monday when it is the first
// column of that month, 0 otherwise -- localizing the abbreviation is
// the renderer's job, not this data builder's.
type dashboardHeatmap struct {
	Weeks       [][7]heatmapDay
	MonthStarts []time.Month
	ActiveDays  int
	MaxCost     float64
}

// mondayOf returns d's calendar date (time stripped) truncated back to
// the Monday of its week.
func mondayOf(d time.Time) time.Time {
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	offset := (int(d.Weekday()) + 6) % 7 // Monday=0 .. Sunday=6
	return d.AddDate(0, 0, -offset)
}

// HeatmapMonthAbbrev are 3-4 letter month labels for the heatmap's
// month row, localized (card: "month labels above" the grid).
var HeatmapMonthAbbrev = map[i18n.Lang][12]string{
	i18n.EN: {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	i18n.FR: {"janv", "f\u00e9vr", "mars", "avr", "mai", "juin", "juil", "ao\u00fbt", "sept", "oct", "nov", "d\u00e9c"},
}

// buildActivityHeatmap builds the Activite panel's grid from dayCost
// (day -> cost, already computed the same way panel C's rows are --
// see dayCostMap), ending on the calendar week containing `now` (so
// re-running later in the same week does not shift columns around),
// weeks wide. A day strictly after `now` is always empty: the grid
// never shows data from the future even mid-week.
func buildActivityHeatmap(dayCost map[string]float64, now time.Time, weeks int) dashboardHeatmap {
	var h dashboardHeatmap
	h.Weeks = make([][7]heatmapDay, weeks)
	h.MonthStarts = make([]time.Month, weeks)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	lastMonday := mondayOf(now)
	firstMonday := lastMonday.AddDate(0, 0, -7*(weeks-1))

	prevMonth := time.Month(0)
	for w := 0; w < weeks; w++ {
		weekMonday := firstMonday.AddDate(0, 0, 7*w)
		if weekMonday.Month() != prevMonth {
			h.MonthStarts[w] = weekMonday.Month()
			prevMonth = weekMonday.Month()
		}
		for d := 0; d < 7; d++ {
			date := weekMonday.AddDate(0, 0, d)
			cell := heatmapDay{Date: date.Format("2006-01-02")}
			if !date.After(today) {
				if cost, ok := dayCost[cell.Date]; ok {
					cell.Cost = cost
					cell.HasData = true
					h.ActiveDays++
					if cost > h.MaxCost {
						h.MaxCost = cost
					}
				}
			}
			h.Weeks[w][d] = cell
		}
	}
	return h
}

// heatmapShade buckets cost into one of 4 quartile shades (0=lightest
// orange .. 3=darkest) against the heatmap's own max cost, or -1 for
// an empty cell (no data, or cost <= 0, or max <= 0): the card's "4
// orange shades by daily cost + empty dim cell".
func heatmapShade(cost, max float64) int {
	if cost <= 0 || max <= 0 {
		return -1
	}
	frac := cost / max
	switch {
	case frac < 0.25:
		return 0
	case frac < 0.5:
		return 1
	case frac < 0.75:
		return 2
	default:
		return 3
	}
}

// dashboardMeasuredSource is the short, human-readable source path shown
// in the "Mesure" panel: a stable "~/..." shorthand, not the real
// (possibly test-fixture) path on disk, since the panel's point is to
// name *what* is read, not resolve the current user's home directory.
var dashboardMeasuredSource = map[string]string{
	string(agents.ClaudeCode): "~/.claude/projects/*/*.jsonl",
	string(agents.Codex):      "~/.codex/sessions/**/rollout-*.jsonl",
	string(agents.Copilot):    "~/.copilot/session-store.db",
	string(agents.Hermes):     "~/.hermes/state.db",
	string(agents.GeminiCLI):  "~/.gemini",
}

// dashboardTTLFindingThreshold mirrors the old Summary tab's
// ttlActionThreshold: a saving estimate below 5% is not worth surfacing
// as a finding.
const dashboardTTLFindingThreshold = 0.05

// dashboardDayPanelMax caps the "Par jour" panel at the most recent N
// days, like CodeBurn's own Daily Activity list.
const dashboardDayPanelMax = 10

// cacheResultFor finds agent's own non-skipped AgentCacheResult among
// the already-loaded cache results.
func cacheResultFor(cache []AgentCacheResult, agent string) (AgentCacheResult, bool) {
	for _, a := range cache {
		if a.Agent == agent && !a.Skipped {
			return a, true
		}
	}
	return AgentCacheResult{}, false
}

// buildDashboard aggregates every already-computed result
// (collectCache, collectTTL, the fixer finding, the per-day history
// loadCmd already loads for the Cache panel's sparkline) into the
// dashboard's data model. scopeAgent is "" for the combined view ('a'
// toggles it) or one agent kind to scope the hero/day/model panels to.
// Panel B (per-agent cache rate) always lists every detected agent
// regardless of scope, since comparing agents is its entire purpose.
func buildDashboard(
	cache []AgentCacheResult, ttlResults []AgentTTLResult,
	fixFinding fixer.Finding, fixHasFinding bool,
	dailyHermes []hermesusage.DailyRow, dailyClaude []claudecode.DailyRow,
	dailyHermesLong []hermesusage.DailyRow, now time.Time, windowDays int,
	hermesCompare, claudeCompare comparisonResult,
	skipped []skippedAgent, scopeAgent string, lang i18n.Lang,
) dashboardData {
	var d dashboardData
	d.ScopeAgent = scopeAgent

	var readTok, writeTok, inputTok, outputTok, calls, sessions int64
	var cost, costWithout float64
	var anyCostKnown bool
	for _, a := range cache {
		if a.Skipped {
			continue
		}
		if scopeAgent != "" && a.Agent != scopeAgent {
			continue
		}
		readTok += a.CacheReadTokens
		writeTok += a.CacheWriteTokens
		inputTok += a.InputTokens
		outputTok += a.OutputTokens
		calls += a.Calls
		sessions += a.Sessions
		if len(a.USD) > 0 {
			anyCostKnown = true
			cost += totalCacheUSD(a)
			costWithout += costWithoutCache(a)
		} else {
			d.HeroCostExcluded = append(d.HeroCostExcluded, a.Agent)
		}
	}
	d.HeroCost = cost
	d.HeroCostKnown = anyCostKnown
	d.HeroSaved = costWithout - cost
	if denom := readTok + writeTok; denom > 0 {
		d.HeroHitRate = float64(readTok) / float64(denom)
	}
	d.HeroCalls, d.HeroSessions = calls, sessions
	d.TokRead, d.TokWrite, d.TokInput, d.TokOutput = readTok, writeTok, inputTok, outputTok

	switch scopeAgent {
	case string(agents.Hermes):
		d.CompareLine = compareLineFor(hermesCompare, lang)
	case string(agents.ClaudeCode):
		d.CompareLine = compareLineFor(claudeCompare, lang)
	}

	for _, a := range cache {
		if a.Skipped {
			continue
		}
		row := dashboardAgentRow{Agent: a.Agent, HitRate: a.HitRate, Calls: a.Calls}
		if len(a.USD) > 0 {
			row.CostKnown = true
			row.Cost = totalCacheUSD(a)
			row.Saved = dashboardSaved(a)
		} else {
			d.AnyUnknownPrice = true
		}
		d.AgentRows = append(d.AgentRows, row)
	}

	d.DayRows, d.NoDailyNote = buildDayRows(dailyHermes, dailyClaude, scopeAgent, lang)
	d.MiniYesterday, d.MiniYesterdayKnown, d.MiniAvg7, d.MiniAvg7Known, d.MiniVsYesterdayPct, d.MiniVsYesterdayKnown = buildHeroMiniStats(d.DayRows)
	d.Heatmap = buildActivityHeatmap(dayCostMap(dailyHermesLong, dailyClaude, scopeAgent), now, dashboardHeatmapWeeks)
	d.Histogram = buildHistogram(dayCostMap(dailyHermes, dailyClaude, scopeAgent), now, windowDays)

	for _, a := range cache {
		if a.Skipped || (scopeAgent != "" && a.Agent != scopeAgent) {
			continue
		}
		for _, mu := range a.ModelBreakdown {
			d.ModelRows = append(d.ModelRows, dashboardModelRow{Agent: a.Agent, Model: mu.Model, HitRate: mu.HitRate, Calls: mu.Calls})
		}
	}
	if len(d.ModelRows) == 0 {
		d.ModelNote = i18n.T(lang, "dashboard.no_model_data")
	}

	d.Findings = buildFindings(cache, ttlResults, fixFinding, fixHasFinding, lang)
	d.Efficiency = buildEfficiency(cache, ttlResults, fixHasFinding, scopeAgent, d.HeroHitRate)
	d.Measured = buildMeasured(cache, lang)
	if len(skipped) > 0 {
		names := make([]string, len(skipped))
		for i, s := range skipped {
			names[i] = string(s.Kind)
		}
		d.IgnoredNote = i18n.T(lang, "dashboard.ignored_note", strings.Join(names, ", "))
	}
	return d
}

// Efficiency score weights (card: "Formula as named constants,
// documented in `?` help, unit tested"). Together they weight three
// already-computed metrics into one 0-100 score: cache hit rate (the
// biggest cost lever), the share of calls free of an abnormal warm
// rebuild, and whether the cache TTL is already well tuned (no
// actionable finding left to apply). See the "dashboard.efficiency_help"
// catalog entry for the plain-words version shown in `?` help.
const (
	efficiencyWeightCacheRate = 0.60
	efficiencyWeightNoRebuild = 0.25
	efficiencyWeightTTLTuned  = 0.15
)

// dashboardEfficiency is panel "Efficacite"'s computed score/letter
// plus the three labelled bars shown under it.
type dashboardEfficiency struct {
	Score        int // 0-100
	Letter       string
	CacheRatePct float64 // 0-100
	NoRebuildPct float64 // 0-100
	TTLWellTuned bool
}

// efficiencyGradeBands maps the minimum score for each letter, highest
// first: the first band whose Min the score meets or exceeds wins.
var efficiencyGradeBands = []struct {
	Min    int
	Letter string
}{
	{97, "A+"}, {93, "A"}, {90, "A-"},
	{87, "B+"}, {83, "B"}, {80, "B-"},
	{77, "C+"}, {73, "C"}, {70, "C-"},
	{67, "D+"}, {60, "D"},
}

// efficiencyLetter maps a 0-100 score to its letter grade (A+..F),
// using efficiencyGradeBands; anything below the lowest band (60) is F.
func efficiencyLetter(score int) string {
	for _, b := range efficiencyGradeBands {
		if score >= b.Min {
			return b.Letter
		}
	}
	return "F"
}

// buildEfficiency computes panel "Efficacite" from metrics every other
// panel already computes -- never a new detector: heroHitRate (the
// same cache hit rate the hero figures and panel B show), the share of
// in-scope calls that did NOT suffer an abnormal warm rebuild
// (AgentCacheResult.WarmRebuilds), and whether the TTL is already well
// tuned (no pending Hermes config fix, and no per-agent TTL finding
// above dashboardTTLFindingThreshold -- the same threshold the "A
// ameliorer" panel uses to decide a TTL saving is worth surfacing).
func buildEfficiency(cache []AgentCacheResult, ttlResults []AgentTTLResult, fixHasFinding bool, scopeAgent string, heroHitRate float64) dashboardEfficiency {
	var calls, rebuilds int64
	for _, a := range cache {
		if a.Skipped || (scopeAgent != "" && a.Agent != scopeAgent) {
			continue
		}
		calls += a.Calls
		rebuilds += int64(a.WarmRebuilds)
	}
	noRebuildRate := 1.0
	if calls > 0 {
		noRebuildRate = 1 - float64(rebuilds)/float64(calls)
		if noRebuildRate < 0 {
			noRebuildRate = 0
		}
	}

	ttlWellTuned := !fixHasFinding
	if ttlWellTuned {
		for _, t := range ttlResults {
			if t.Skipped || (scopeAgent != "" && t.Agent != scopeAgent) {
				continue
			}
			if t.EstimatedSaving > dashboardTTLFindingThreshold {
				ttlWellTuned = false
				break
			}
		}
	}
	ttlScore := 0.0
	if ttlWellTuned {
		ttlScore = 100
	}

	cacheScore := heroHitRate * 100
	rebuildScore := noRebuildRate * 100
	raw := efficiencyWeightCacheRate*cacheScore + efficiencyWeightNoRebuild*rebuildScore + efficiencyWeightTTLTuned*ttlScore
	score := int(math.Round(raw))
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return dashboardEfficiency{
		Score: score, Letter: efficiencyLetter(score),
		CacheRatePct: cacheScore, NoRebuildPct: rebuildScore, TTLWellTuned: ttlWellTuned,
	}
}

// compareLineFor renders a dim one-line previous-period comparison for
// the hero section (only shown when a single agent is selected -- see
// buildDashboard): "" when there is no previous-period data, or when
// the swing is under one point (not worth a line of chrome for).
func compareLineFor(c comparisonResult, lang i18n.Lang) string {
	if !c.HasPrevious {
		return ""
	}
	abs := c.DeltaPoints
	if abs < 0 {
		abs = -abs
	}
	if abs < 1 {
		return ""
	}
	pts := fmt.Sprintf("%.0f", abs)
	if c.DeltaPoints > 0 {
		return i18n.T(lang, "dashboard.compare_up", pts)
	}
	return i18n.T(lang, "dashboard.compare_down", pts)
}

// histogramBar is one day's bar in the "Depenses quotidiennes" panel.
type histogramBar struct {
	Day  string
	Cost float64
}

// dashboardHistogram is panel "Depenses quotidiennes"'s computed
// data: Bars oldest-first (left-to-right chronological reading), plus
// the title row's chips (card: "Moy./jour X $  Pic X $ . date  Hier X $").
type dashboardHistogram struct {
	Bars           []histogramBar
	AvgPerDay      float64
	Peak           float64
	PeakDay        string
	Yesterday      float64
	YesterdayKnown bool
}

// buildHistogram builds panel "Depenses quotidiennes" from dayCost
// (the same day->cost map the Activite heatmap uses, see dayCostMap)
// over exactly `days` calendar days ending on `now`'s day (the
// dashboard's own selected window, card: no separate lookback here
// unlike the heatmap). A day missing from dayCost is a real 0, not
// unknown: only the window itself being empty would mean "no data",
// and even then every bar still renders at 0 rather than being
// dropped, so the window always has exactly `days` columns.
func buildHistogram(dayCost map[string]float64, now time.Time, days int) dashboardHistogram {
	var h dashboardHistogram
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := today.AddDate(0, 0, -(days - 1))

	var sum float64
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		day := d.Format("2006-01-02")
		cost := dayCost[day]
		h.Bars = append(h.Bars, histogramBar{Day: day, Cost: cost})
		sum += cost
		if cost > h.Peak {
			h.Peak = cost
			h.PeakDay = day
		}
	}
	if days > 0 {
		h.AvgPerDay = sum / float64(days)
	}
	if n := len(h.Bars); n > 0 {
		h.Yesterday = h.Bars[n-1].Cost
		h.YesterdayKnown = true
	}
	return h
}

// dayAccum sums raw token counts for one calendar day across every
// agent contributing to it, so cost/hit-rate are computed once from the
// combined totals instead of trying to average two already-computed
// per-agent percentages.
type dayAccum struct{ read, write, input, output int64 }

// accumulateDayDay combines dailyHermes/dailyClaude into a per-day
// dayAccum map, scoped to scopeAgent ("" = every agent with daily
// history). Shared by buildDayRows (panel "Par jour"), the Activite
// heatmap, and the "Depenses quotidiennes" histogram -- one
// accumulation, several different views over it, never a second
// parser.
func accumulateDayDay(dailyHermes []hermesusage.DailyRow, dailyClaude []claudecode.DailyRow, scopeAgent string) map[string]*dayAccum {
	acc := map[string]*dayAccum{}
	get := func(day string) *dayAccum {
		a := acc[day]
		if a == nil {
			a = &dayAccum{}
			acc[day] = a
		}
		return a
	}
	if scopeAgent == "" || scopeAgent == string(agents.Hermes) {
		for _, r := range dailyHermes {
			a := get(r.Day)
			a.read += r.CacheRead
			a.write += r.CacheWrite
			a.input += r.Input
			a.output += r.Output
		}
	}
	if scopeAgent == "" || scopeAgent == string(agents.ClaudeCode) {
		for _, r := range dailyClaude {
			a := get(r.Day)
			a.read += r.Read
			a.write += r.Create
			a.input += r.Input
			a.output += r.Output
		}
	}
	return acc
}

// dayAccumCost prices one day's accumulated tokens through usdSplit --
// the exact same pricing table and formula the Cache panel uses.
func dayAccumCost(a *dayAccum) float64 {
	usd, _ := usdSplit(a.input, a.output, a.read, a.write)
	return usd["input"] + usd["output"] + usd["cache_read"] + usd["cache_write"]
}

// dayCostMap builds a day -> cost map from dailyHermes/dailyClaude,
// scoped to scopeAgent, for the Activite heatmap and the "Depenses
// quotidiennes" histogram (both want every day, not just the most
// recent dashboardDayPanelMax panel C caps at).
func dayCostMap(dailyHermes []hermesusage.DailyRow, dailyClaude []claudecode.DailyRow, scopeAgent string) map[string]float64 {
	acc := accumulateDayDay(dailyHermes, dailyClaude, scopeAgent)
	out := make(map[string]float64, len(acc))
	for day, a := range acc {
		out[day] = dayAccumCost(a)
	}
	return out
}

// buildDayRows combines the already-loaded per-day histories
// (dailyHermes/dailyClaude -- the same rows the Cache panel's
// sparkline uses) into the "Par jour" panel's rows, newest first,
// capped at dashboardDayPanelMax. scopeAgent restricts the combination
// to one agent; an agent with no daily history at all (codex, copilot,
// gemini-cli) returns a dim explanatory note instead of an empty panel.
func buildDayRows(dailyHermes []hermesusage.DailyRow, dailyClaude []claudecode.DailyRow, scopeAgent string, lang i18n.Lang) ([]dashboardDayRow, string) {
	if scopeAgent != "" && scopeAgent != string(agents.Hermes) && scopeAgent != string(agents.ClaudeCode) {
		return nil, i18n.T(lang, "dashboard.no_daily_data")
	}
	acc := accumulateDayDay(dailyHermes, dailyClaude, scopeAgent)
	days := make([]string, 0, len(acc))
	for day := range acc {
		days = append(days, day)
	}
	sort.Strings(days)

	var rows []dashboardDayRow
	for i := len(days) - 1; i >= 0 && len(rows) < dashboardDayPanelMax; i-- {
		day := days[i]
		a := acc[day]
		cost := dayAccumCost(a)
		var hitRate float64
		if denom := a.read + a.write; denom > 0 {
			hitRate = float64(a.read) / float64(denom)
		}
		rows = append(rows, dashboardDayRow{Day: day, Cost: cost, HitRate: hitRate})
	}
	if len(rows) == 0 {
		return nil, i18n.T(lang, "dashboard.no_daily_data")
	}
	return rows, ""
}

// buildHeroMiniStats computes the hero section's 3 mini-stats (card
// 1a) from dayRows (buildDayRows' output: newest-first, capped at
// dashboardDayPanelMax days): yesterday's cost (the newest row),
// the average cost over up to the 7 most recent days, and the percent
// swing of yesterday vs the day before it. Each *Known flag is false
// when there is not enough history to compute its value -- never an
// invented number.
func buildHeroMiniStats(dayRows []dashboardDayRow) (yesterday float64, yesterdayKnown bool, avg7 float64, avg7Known bool, vsYesterdayPct float64, vsYesterdayKnown bool) {
	if len(dayRows) == 0 {
		return 0, false, 0, false, 0, false
	}
	yesterday = dayRows[0].Cost
	yesterdayKnown = true

	n := len(dayRows)
	if n > 7 {
		n = 7
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += dayRows[i].Cost
	}
	avg7 = sum / float64(n)
	avg7Known = true

	if len(dayRows) >= 2 && dayRows[1].Cost > 0 {
		vsYesterdayPct = (dayRows[0].Cost - dayRows[1].Cost) / dayRows[1].Cost * 100
		vsYesterdayKnown = true
	}
	return yesterday, yesterdayKnown, avg7, avg7Known, vsYesterdayPct, vsYesterdayKnown
}

// buildFindings collects every already-detected anomaly/recommendation
// into the "A ameliorer" panel's ranked rows: the Hermes config fix
// (fixer.DetectHermesCacheTTLFinding), a worthwhile TTL saving per
// agent (collectTTL's own EstimatedSaving), abnormal warm-cache
// rebuilds (claude-code's WarmRebuildDetails), and a Copilot model
// switch emptying the cache (len(ModelBreakdown) > 1). Nothing here is
// a new detector: every finding reuses a number ats already computes
// elsewhere, ranked biggest-estimated-impact-first by rankFindings.
func buildFindings(cache []AgentCacheResult, ttlResults []AgentTTLResult, fixFinding fixer.Finding, fixHasFinding bool, lang i18n.Lang) []dashboardFinding {
	var out []dashboardFinding
	if fixHasFinding {
		out = append(out, dashboardFinding{
			Label:     i18n.T(lang, "dashboard.finding_fix_hermes"),
			DetailKey: "5",
			// A config fix is free to apply and unlocks the TTL saving
			// below it would otherwise mask; always rank it first.
			Magnitude: 1e12,
		})
	}
	for _, t := range ttlResults {
		if t.Skipped || t.EstimatedSaving <= dashboardTTLFindingThreshold {
			continue
		}
		f := dashboardFinding{Label: i18n.T(lang, "dashboard.finding_ttl", t.Agent), DetailKey: "4"}
		if a, ok := cacheResultFor(cache, t.Agent); ok && len(a.USD) > 0 {
			amount := totalCacheUSD(a) * t.EstimatedSaving
			f.Amount = render.FormatUSDLang(amount, lang)
			f.Magnitude = amount
		} else if tok := int64(float64(t.TotalWriteTokens) * t.EstimatedSaving); tok > 0 {
			f.Amount = render.FormatCountLang(tok, lang) + " tok"
			f.Magnitude = float64(tok)
		}
		// Dedupe by (agent, kind) (card point 7): the Hermes config
		// fix above and this per-agent TTL saving describe the exact
		// same root cause ("this agent's cache TTL is badly tuned")
		// when they name the same agent -- merge this finding's $/tok
		// estimate into the existing fix finding instead of listing
		// the same problem twice.
		if fixHasFinding && t.Agent == fixFinding.Agent {
			for i := range out {
				if out[i].DetailKey == "5" {
					out[i].Amount = f.Amount
					break
				}
			}
			continue
		}
		out = append(out, f)
	}
	for _, a := range cache {
		if a.Skipped || a.WarmRebuilds == 0 {
			continue
		}
		var tok int64
		for _, w := range a.WarmRebuildDetails {
			tok += w.Tokens
		}
		amountUSD := float64(tok) * hermesusage.PriceUSDPerMTok["cache_write"] / 1e6
		out = append(out, dashboardFinding{
			Label:     i18n.T(lang, "dashboard.finding_warm_rebuilds", a.Agent),
			Amount:    render.FormatUSDLang(amountUSD, lang),
			DetailKey: "2",
			Magnitude: amountUSD,
		})
	}
	for _, a := range cache {
		// Copilot-specific: Copilot auto-routes between models
		// mid-session, which invalidates the prompt cache on every
		// switch (the card's actual anomaly). claude-code also
		// reports a ModelBreakdown (since 7e3d4d1) but never
		// auto-routes -- it is just a usage-by-model report, with no
		// equivalent cache-emptying behavior, so this finding must
		// stay scoped to agents.Copilot and never fire for it or any
		// other agent just because len(ModelBreakdown) > 1.
		if a.Skipped || a.Agent != string(agents.Copilot) || len(a.ModelBreakdown) <= 1 {
			continue
		}
		out = append(out, dashboardFinding{
			Label:     i18n.T(lang, "dashboard.finding_copilot_switch"),
			DetailKey: "2",
			Magnitude: float64(a.CacheWriteTokens),
		})
	}
	return rankFindings(out)
}

// buildMeasured builds the "Mesure" panel's rows: what ats read for
// each detected agent and what it knowingly does not report
// (CacheWriteUnknown for Codex, CostUnknown for Codex/Copilot).
func buildMeasured(cache []AgentCacheResult, lang i18n.Lang) []dashboardMeasuredRow {
	var out []dashboardMeasuredRow
	for _, a := range cache {
		if a.Skipped {
			continue
		}
		row := dashboardMeasuredRow{
			Agent: a.Agent, Source: dashboardMeasuredSource[a.Agent],
			Period:   i18n.T(lang, "card.last_days", a.Days),
			Sessions: a.Sessions, SessionsNoUsage: a.SessionsWithoutUsage,
		}
		switch {
		case a.CacheWriteUnknown:
			row.Note = i18n.T(lang, "dashboard.not_measured_cache_write", a.Agent)
		case a.CostUnknown:
			row.Note = i18n.T(lang, "dashboard.not_measured_price")
		}
		out = append(out, row)
	}
	return out
}
