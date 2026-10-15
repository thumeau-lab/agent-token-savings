package cliapp

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// TestRenderPanelMeasuredUsesPlainSessionsWording pins the card's wording
// fix: "337 sessions", never the old "sessions=337" key=value shorthand.
func TestRenderPanelMeasuredUsesPlainSessionsWording(t *testing.T) {
	d := dashboardData{Measured: []dashboardMeasuredRow{
		{Agent: "hermes", Source: "~/.hermes/state.db", Period: "7 derniers jours", Sessions: 337},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelMeasured(d, r, i18n.FR, 70), "\n")
	if strings.Contains(joined, "sessions=337") {
		t.Fatalf("still using the old sessions=N format: %q", joined)
	}
	if !strings.Contains(joined, "337 sessions") {
		t.Fatalf("want plain-words %q, got %q", "337 sessions", joined)
	}
}

// TestRenderPanelMeasuredWrapsLongLinesInsteadOfTruncating is the card
// point 6 regression test: the codex row's line ("codex:
// ~/.codex/sessions/**/rollout-*.jsonl (last 7 days, 26 sessions), 4
// without usage") is longer than a row4 third-width panel at 200 cols
// -- it must wrap onto a continuation line inside the box, never cut
// short with an ellipsis.
func TestRenderPanelMeasuredWrapsLongLinesInsteadOfTruncating(t *testing.T) {
	d := dashboardData{Measured: []dashboardMeasuredRow{
		{
			Agent: "codex", Source: dashboardMeasuredSource[string(agents.Codex)],
			Period: "last 7 days", Sessions: 26, SessionsNoUsage: 4,
		},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	const width = 70
	lines := renderPanelMeasured(d, r, i18n.EN, width)
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "\u2026") {
		t.Fatalf("renderPanelMeasured(width=%d) = %q, the codex line was truncated with an ellipsis, want it wrapped instead", width, joined)
	}
	if !strings.Contains(joined, dashboardMeasuredSource[string(agents.Codex)]) {
		t.Fatalf("renderPanelMeasured(width=%d) = %q, missing the full source path (never cut)", width, joined)
	}
	if !strings.Contains(joined, "4") || !strings.Contains(joined, "without usage") {
		t.Fatalf("renderPanelMeasured(width=%d) = %q, missing the \"without usage\" detail (never cut)", width, joined)
	}
	inner := width - 4
	for i, line := range lines {
		if w := len([]rune(line)); w > inner {
			t.Fatalf("renderPanelMeasured(width=%d) line %d is %d cols wide, want <= %d (wrapped to the box's own inner width): %q", width, i, w, inner, line)
		}
	}
}

// TestRenderPanelByAgentNeverWrapsWithFourDigitCostOrSavedAmounts is
// the card point 1 regression test: a cost with 4 digits (e.g.
// "1812,36 $" in fr, 9 columns) overflowed the old fixed 8-column cost
// field, making lipgloss wrap the row onto a second visual line and
// misplace the box's right border. Columns must size from the widest
// formatted value actually present, never wrap a data row.
func TestRenderPanelByAgentNeverWrapsWithFourDigitCostOrSavedAmounts(t *testing.T) {
	d := dashboardData{AgentRows: []dashboardAgentRow{
		{Agent: "hermes", HitRate: 0.92, Calls: 13_100, CostKnown: true, Cost: 1812.36, Saved: 6769.37},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelByAgent(d, r, i18n.FR, 200)
	if len(lines) != 2 {
		t.Fatalf("renderPanelByAgent() = %v, want exactly 2 lines (header + 1 data row, no wrap)", lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "1812,36 $") {
		t.Fatalf("renderPanelByAgent() = %q, missing the full cost %q (never wrapped)", joined, "1812,36 $")
	}
	if !strings.Contains(joined, "6769,37 $") {
		t.Fatalf("renderPanelByAgent() = %q, missing the full saved amount %q (never wrapped)", joined, "6769,37 $")
	}
}

// TestRenderPanelByAgentHasColumnHeadersAndSavedColumn pins the card's
// panel B fix: an explicit header row (agent, cache, appels, cout,
// economise) and a "saved" column, not just bare data rows.
func TestRenderPanelByAgentHasColumnHeadersAndSavedColumn(t *testing.T) {
	d := dashboardData{AgentRows: []dashboardAgentRow{
		{Agent: "claude-code", HitRate: 0.9, Calls: 100, CostKnown: true, Cost: 12.5, Saved: 3.2},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelByAgent(d, r, i18n.FR, 60)
	if len(lines) < 2 {
		t.Fatalf("renderPanelByAgent() = %v, want a header row plus at least one data row", lines)
	}
	header := strings.ToLower(lines[0])
	for _, want := range []string{"agent", "cache", "appels", "co\u00fbt", "\u00e9conomis\u00e9"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header %q missing %q", lines[0], want)
		}
	}
	dataLine := strings.Join(lines[1:], "\n")
	if !strings.Contains(dataLine, "$3.20") && !strings.Contains(dataLine, "3,20") {
		t.Fatalf("data row %q missing the saved amount", dataLine)
	}
}

// TestRenderPanelByAgentNeverTruncatesCostOrSavedAtNarrowWidth pins
// card point 6: at a narrow panel width, the bar/calls columns are
// dropped first (card: "drop columns/text instead of truncating
// numbers"), but the cost and saved ($) columns must always render in
// full.
func TestRenderPanelByAgentNeverTruncatesCostOrSavedAtNarrowWidth(t *testing.T) {
	d := dashboardData{AgentRows: []dashboardAgentRow{
		{Agent: "claude-code", HitRate: 0.868, Calls: 340, CostKnown: true, Cost: 101.30, Saved: 245.86},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelByAgent(d, r, i18n.FR, 49)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "245,86 $") {
		t.Fatalf("renderPanelByAgent(width=49) = %q, the saved amount must never be truncated", joined)
	}
	if !strings.Contains(joined, "101,30 $") {
		t.Fatalf("renderPanelByAgent(width=49) = %q, the cost amount must never be truncated", joined)
	}
	for i, line := range lines {
		if w := len([]rune(line)); w > 45 {
			t.Fatalf("renderPanelByAgent(width=49) line %d is %d cols wide, want <= 45 (the box's inner width): %q", i, w, line)
		}
	}
}

// TestRenderPanelEfficiencyHasNoBlankLineAndShowsBigScore pins card
// point 5: the score renders big (block digits) with its letter next
// to it, and the bars sit right alongside -- no blank filler row
// between the score headline and the bars.
func TestRenderPanelEfficiencyHasNoBlankLineAndShowsBigScore(t *testing.T) {
	d := dashboardData{Efficiency: dashboardEfficiency{
		Score: 80, Letter: "B-", CacheRatePct: 89, NoRebuildPct: 100, TTLWellTuned: false,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelEfficiency(d, r, i18n.FR, true)
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			t.Fatalf("renderPanelEfficiency() line %d is blank, want no blank rows between the score and the bars: %v", i, lines)
		}
	}
	joined := strings.Join(lines, "\n")
	// The score's block-digit banner draws "8" and "0" as 3-row glyphs
	// built from the same box-drawing/half-block runes BlockDigits
	// always uses (see render.BlockDigits) -- distinct from the plain
	// bars/borders this panel also draws.
	if !strings.ContainsAny(joined, "\u2580\u2584") {
		t.Fatalf("renderPanelEfficiency() = %q, want the score rendered big (block digits)", joined)
	}
	if !strings.Contains(joined, "B-") {
		t.Fatalf("renderPanelEfficiency() = %q, missing the letter grade next to the score", joined)
	}
}

// TestRenderPanelImproveNeverTruncatesTheAmountAtNarrowWidth pins card
// point 6: at a narrow panel width, the label shrinks (or the detail
// key is dropped) but the $/token amount itself must always render in
// full -- never cut with an ellipsis.
func TestRenderPanelImproveNeverTruncatesTheAmountAtNarrowWidth(t *testing.T) {
	d := dashboardData{Findings: []dashboardFinding{
		{Label: "TTL de cache mal regle sur un nom d'agent tres tres long", Amount: "1 234,56 $", DetailKey: "4"},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderPanelImprove(d, r, i18n.FR, 45)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "1 234,56 $") {
		t.Fatalf("renderPanelImprove(width=45) = %q, the amount must never be truncated", joined)
	}
	for i, line := range lines {
		if w := len([]rune(line)); w > 41 {
			t.Fatalf("renderPanelImprove(width=45) line %d is %d cols wide, want <= 41 (the box's inner width): %q", i, w, line)
		}
	}
}

// TestRenderPanelEfficiencyShowsScoreLetterAndBars pins panel
// "Efficacite"'s content: the letter grade and the three labelled
// bars (card: "Taux de cache 91%", "Sans reconstruction anormale x%",
// "TTL bien regle oui/non"). The score itself (card point 5) now
// renders as a block-digit banner, not literal digit characters --
// see TestRenderPanelEfficiencyHasNoBlankLineAndShowsBigScore for
// that assertion.
func TestRenderPanelEfficiencyShowsScoreLetterAndBars(t *testing.T) {
	d := dashboardData{Efficiency: dashboardEfficiency{
		Score: 97, Letter: "A+", CacheRatePct: 95, NoRebuildPct: 100, TTLWellTuned: true,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelEfficiency(d, r, i18n.FR, true), "\n")
	for _, want := range []string{"A+", "95", "100", "oui"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("renderPanelEfficiency() = %q, missing %q", joined, want)
		}
	}
}

func TestRenderPanelEfficiencyShowsNonForUntunedTTL(t *testing.T) {
	d := dashboardData{Efficiency: dashboardEfficiency{
		Score: 62, Letter: "D", CacheRatePct: 70, NoRebuildPct: 80, TTLWellTuned: false,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelEfficiency(d, r, i18n.FR, true), "\n")
	if !strings.Contains(joined, "non") {
		t.Fatalf("renderPanelEfficiency() = %q, want it to say the TTL is not well tuned", joined)
	}
}

// TestRenderHeroMiniStatsShowsCallsSessionsAndSwing pins card 1a: a
// calls/sessions line, then yesterday/7-day-avg/vs-yesterday -- each
// *Known flag gating its own segment independently.
func TestRenderHeroMiniStatsShowsCallsSessionsAndSwing(t *testing.T) {
	d := dashboardData{
		HeroCalls: 340, HeroSessions: 12,
		MiniYesterday: 11.90, MiniYesterdayKnown: true,
		MiniAvg7: 10.30, MiniAvg7Known: true,
		MiniVsYesterdayPct: 19.0, MiniVsYesterdayKnown: true,
	}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderHeroMiniStats(d, r, i18n.FR), "\n")
	for _, want := range []string{"340", "12", "appels", "sessions", "11,90", "10,30", "+19"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("renderHeroMiniStats() = %q, missing %q", joined, want)
		}
	}
}

// TestRenderHeroMiniStatsOmitsUnknownSwing: an agent with no daily
// history at all (buildHeroMiniStats' "never invent a 0" rule) must
// not show a fabricated yesterday/avg/swing line.
func TestRenderHeroMiniStatsOmitsUnknownSwing(t *testing.T) {
	d := dashboardData{HeroCalls: 50, HeroSessions: 8}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	lines := renderHeroMiniStats(d, r, i18n.FR)
	if len(lines) != 1 {
		t.Fatalf("renderHeroMiniStats() = %v, want exactly 1 line (calls/sessions only) when nothing else is known", lines)
	}
}

// TestFormatSignedPercentLangAddsPlusForPositive pins the explicit "+"
// sign the card's example shows ("vs hier +19 %"); FormatPercentLang
// alone never adds one for a positive value.
func TestFormatSignedPercentLangAddsPlusForPositive(t *testing.T) {
	if got := formatSignedPercentLang(19, i18n.EN); got != "+19%" {
		t.Fatalf("formatSignedPercentLang(19) = %q, want %q", got, "+19%")
	}
	if got := formatSignedPercentLang(-5, i18n.EN); got != "-5%" {
		t.Fatalf("formatSignedPercentLang(-5) = %q, want %q", got, "-5%")
	}
}

// TestRenderPanelBoxTruncatesOverlongLines guards against the overflow
// bug the narrower row4 columns exposed: a content line wider than the
// box's inner width must be truncated to fit, never wrapped onto an
// extra visual row that breaks out past the right border (lipgloss's
// own Width() wraps instead of truncating oversized content).
func TestRenderPanelBoxTruncatesOverlongLines(t *testing.T) {
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	longLine := "claude-code: ~/.claude/projects/*/*.jsonl (last 7 days, 12 sessions)"
	out := renderPanelBox(r, "Measured", []string{longLine}, 40, nil)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 { // top border, 1 content row, bottom border
		t.Fatalf("renderPanelBox() with a 40-col box and one long line produced %d lines, want 3 (no wrap):\n%s", len(lines), out)
	}
	for i, line := range lines {
		if w := len([]rune(line)); w > 40 {
			t.Fatalf("renderPanelBox() line %d is %d cols wide, want <= 40:\n%q", i, w, line)
		}
	}
}

// TestRenderDashboardHeroShowsBlockDigitsAtColumnWidth60 pins the card's
// fix: "block digits must show for the hero whenever its column is
// >=60 wide" -- the hero is now rendered inside a half-width column by
// the wide/medium layouts, so the threshold must compare against the
// column width it actually receives (not the raw terminal width).
func TestRenderDashboardHeroShowsBlockDigitsAtColumnWidth60(t *testing.T) {
	d := dashboardData{
		HeroCost: 126.70, HeroSaved: 359.86, HeroCostKnown: true, HeroHitRate: 0.888,
		HeroCalls: 1350, HeroSessions: 65,
	}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboardHero(d, r, 60, i18n.EN, true)
	if !strings.ContainsAny(out, "\u2580\u2584") {
		t.Fatalf("renderDashboardHero(width=60, allowBlockDigits=true) has no block-digit glyphs, want them to show at >=60 cols:\n%s", out)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := len([]rune(line)); w > 60 {
			t.Fatalf("renderDashboardHero(width=60) line %d is %d cols wide, want <= 60:\n%q", i, w, line)
		}
	}
}

// TestRenderDashboardHeroHidesBlockDigitsBelowColumnWidth60: just under
// the threshold, the plain-words fallback line must still be used.
func TestRenderDashboardHeroHidesBlockDigitsBelowColumnWidth60(t *testing.T) {
	d := dashboardData{HeroCost: 126.70, HeroSaved: 359.86, HeroCostKnown: true, HeroHitRate: 0.888}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboardHero(d, r, 59, i18n.EN, true)
	if strings.ContainsAny(out, "\u2580\u2584") {
		t.Fatalf("renderDashboardHero(width=59, allowBlockDigits=true) rendered block digits, want the plain fallback below the 60-col threshold:\n%s", out)
	}
}

// TestRenderDashboardHeroFitsWidth guards the sibling overflow bug:
// the hero's plain-fallback line (and every line under it) was never
// width-constrained because it always used to get the full terminal
// width; now that the wide/medium layouts confine it to half/a column,
// it must fit like every other panel instead of overflowing past its
// allotted column when joined side by side with Efficacite.
func TestRenderDashboardHeroFitsWidth(t *testing.T) {
	d := dashboardData{
		HeroCost: 126.70, HeroSaved: 359.86, HeroCostKnown: true, HeroHitRate: 0.888,
		HeroCalls: 1350, HeroSessions: 65,
		MiniYesterday: 12, MiniYesterdayKnown: true,
		MiniAvg7: 12.27, MiniAvg7Known: true,
		MiniVsYesterdayPct: -4, MiniVsYesterdayKnown: true,
	}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboardHero(d, r, 49, i18n.EN, true)
	for i, line := range strings.Split(out, "\n") {
		if w := len([]rune(line)); w > 49 {
			t.Fatalf("renderDashboardHero(width=49) line %d is %d cols wide, want <= 49:\n%q", i, w, line)
		}
	}
}

// TestRenderPanelActivityMonthLabelsNeverOverlap pins card point 3:
// two month labels close enough together that the first one's text
// would run into the second one's starting column must never produce
// a glued-together run of letters like "juilaout" -- the second label
// is skipped instead (card: "skipped if it would overlap the
// previous one").
func TestRenderPanelActivityMonthLabelsNeverOverlap(t *testing.T) {
	d := dashboardData{Heatmap: dashboardHeatmap{
		Weeks:       make([][7]heatmapDay, 6),
		MonthStarts: []time.Month{0, 0, time.July, time.August, 0, 0},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelActivity(d, r, i18n.FR), "\n")
	if strings.Contains(joined, "juilao\u00fbt") {
		t.Fatalf("renderPanelActivity() = %q, month labels collided into one glued run", joined)
	}
}

// TestRenderPanelActivityShowsBothMonthLabelsWhenFarApart: the skip
// logic must only kick in on genuine overlap, never drop a label that
// has room to breathe.
func TestRenderPanelActivityShowsBothMonthLabelsWhenFarApart(t *testing.T) {
	d := dashboardData{Heatmap: dashboardHeatmap{
		Weeks:       make([][7]heatmapDay, 12),
		MonthStarts: []time.Month{time.July, 0, 0, 0, 0, 0, time.August, 0, 0, 0, 0, 0},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelActivity(d, r, i18n.EN), "\n")
	if !strings.Contains(joined, "Jul") || !strings.Contains(joined, "Aug") {
		t.Fatalf("renderPanelActivity() = %q, want both month labels shown when far apart", joined)
	}
}

// TestRenderPanelActivityShowsDayLabelsAndActiveDaysCount pins card
// point 3's other two asks: lun/mer/ven (Mon/Wed/Fri) labels on the
// left margin, and a "N jours actifs" (active days) line.
func TestRenderPanelActivityShowsDayLabelsAndActiveDaysCount(t *testing.T) {
	d := dashboardData{Heatmap: dashboardHeatmap{
		Weeks:       make([][7]heatmapDay, 4),
		MonthStarts: make([]time.Month, 4),
		ActiveDays:  7,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelActivity(d, r, i18n.FR), "\n")
	for _, want := range []string{"lun", "mer", "ven", "7"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("renderPanelActivity() = %q, missing %q", joined, want)
		}
	}
}

// TestRenderPanelActivityShowsDistinctShades pins card point 3's
// anti-regression check: with costs spanning the full quartile range,
// the rendered grid must show every distinct shade bucket, not one
// uniform glyph everywhere.
func TestRenderPanelActivityShowsDistinctShades(t *testing.T) {
	var week [7]heatmapDay
	week[0] = heatmapDay{Cost: 1, HasData: true}  // shade 0
	week[1] = heatmapDay{Cost: 4, HasData: true}  // shade 1
	week[2] = heatmapDay{Cost: 6, HasData: true}  // shade 2
	week[3] = heatmapDay{Cost: 10, HasData: true} // shade 3 (max)
	d := dashboardData{Heatmap: dashboardHeatmap{
		Weeks:       [][7]heatmapDay{week},
		MonthStarts: []time.Month{0},
		MaxCost:     10,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	joined := strings.Join(renderPanelActivity(d, r, i18n.FR), "\n")
	seen := map[rune]bool{}
	for _, ch := range joined {
		if strings.ContainsRune(".-+*#", ch) {
			seen[ch] = true
		}
	}
	if len(seen) < 4 {
		t.Fatalf("renderPanelActivity() = %q, want at least 4 distinct shade glyphs, got %d (%v)", joined, len(seen), seen)
	}
}

// TestRenderPanelActivityShowsFourDistinctSGRColorsUnder256Profile is
// the card's explicit point 4 test: with costs spanning the full
// quartile range and a colored (non-NoColor) renderer, the 4 shade
// buckets must still resolve to 4 distinct SGR color codes once
// downsampled through a forced 256-color profile (card: "confirm 4
// distinct shades are emitted -- different SGR colors -- for 4 cost
// quartiles with a 256-color profile"), same colorprofile.Writer
// technique as TestDashboardSGRSurvivesForcedColorProfile.
func TestRenderPanelActivityShowsFourDistinctSGRColorsUnder256Profile(t *testing.T) {
	var week [7]heatmapDay
	week[0] = heatmapDay{Cost: 1, HasData: true}  // shade 0
	week[1] = heatmapDay{Cost: 4, HasData: true}  // shade 1
	week[2] = heatmapDay{Cost: 6, HasData: true}  // shade 2
	week[3] = heatmapDay{Cost: 10, HasData: true} // shade 3 (max)
	d := dashboardData{Heatmap: dashboardHeatmap{
		Weeks:       [][7]heatmapDay{week},
		MonthStarts: []time.Month{0},
		MaxCost:     10,
	}}
	r := render.New(render.Options{IsTTY: true, Dark: true})
	joined := strings.Join(renderPanelActivity(d, r, i18n.FR), "\n")

	var buf bytes.Buffer
	w := &colorprofile.Writer{Forward: &buf, Profile: colorprofile.ANSI256}
	if _, err := w.WriteString(joined); err != nil {
		t.Fatalf("WriteString: %v", err)
	}

	cellColor := regexp.MustCompile(`(\x1b\[[0-9;]+m)\x{25a0}`)
	colors := map[string]bool{}
	for _, m := range cellColor.FindAllStringSubmatch(buf.String(), -1) {
		colors[m[1]] = true
	}
	if len(colors) < 4 {
		t.Fatalf("renderPanelActivity() under ANSI256 shows %d distinct cell SGR colors, want >= 4 (one per cost quartile):\n%q", len(colors), buf.String())
	}
}

// TestRenderPanelHistogramChipsLineNeverTruncatesMoneyAtNarrowWidth
// pins card point 6 applied to the histogram's own headline chips
// line (Moy./jour, Pic, Hier): at the real narrow 2-column panel
// width, the generic box-level ellipsis truncation must never eat
// into a money figure -- the chips line must drop a whole field
// (e.g. "Hier ...") before it lets ansi.Truncate cut a $ value in
// half.
func TestRenderPanelHistogramChipsLineNeverTruncatesMoneyAtNarrowWidth(t *testing.T) {
	d := dashboardData{Histogram: dashboardHistogram{
		Bars: []histogramBar{
			{Day: "2026-10-08", Cost: 1}, {Day: "2026-10-14", Cost: 12.54},
		},
		AvgPerDay: 3.51, Peak: 12.54, PeakDay: "2026-10-12", Yesterday: 0, YesterdayKnown: true,
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	box := dashboardHistogramBox(d, r, 49, i18n.FR, 8)
	if !strings.Contains(box, "0,00 $") {
		t.Fatalf("dashboardHistogramBox(width=49) = %q, the Hier amount must never be truncated away", box)
	}
	if strings.Contains(box, "Hier \u2026") || strings.Contains(box, "Hier\u2026") {
		t.Fatalf("dashboardHistogramBox(width=49) = %q, chips line truncated right after \"Hier\"", box)
	}
}

// TestRenderDashboardHeroTokensLineWrapsInsteadOfTruncating is the
// follow-up card's point 3 regression test: at a hero column width
// that is generous overall but still narrower than the full token
// line (e.g. the hero's own ~half-terminal column at 200 cols), the
// dim token-counts line must wrap onto a second line instead of being
// cut with an ellipsis -- there is free space elsewhere on screen, an
// ellipsis just throws information away for nothing.
func TestRenderDashboardHeroTokensLineWrapsInsteadOfTruncating(t *testing.T) {
	d := dashboardData{
		HeroCostKnown: true, HeroCost: 65.59, HeroSaved: 420.11, HeroHitRate: 0.91,
		TokRead: 9_330_000_000, TokWrite: 512_000_000, TokInput: 2_400_000, TokOutput: 530_000,
	}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	const width = 99 // the wide layout's hero column at 200 cols: (200-1)/2
	out := renderDashboardHero(d, r, width, i18n.FR, true)
	if strings.Contains(out, "\u2026") {
		t.Fatalf("renderDashboardHero(width=%d) = %q, the tokens line was truncated with an ellipsis, want it wrapped onto another line instead", width, out)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := len([]rune(line)); w > width {
			t.Fatalf("renderDashboardHero(width=%d) line %d is %d cols wide, want <= %d:\n%q", width, i, w, width, line)
		}
	}
}

// TestRenderDashboardHeroShowsCostExclusionLabelWhenSomeAgentsUnpriced
// is the follow-up card's point C regression test: the hero "cout"
// figure only ever sums the agents with a known price (buildDashboard
// skips CostUnknown agents), so showing the plain "cout" label next
// to it implied the figure covered every detected agent when it did
// not. When d.HeroCostExcluded lists the unpriced agents, the label
// must name them instead of silently under-representing the total.
func TestRenderDashboardHeroShowsCostExclusionLabelWhenSomeAgentsUnpriced(t *testing.T) {
	d := dashboardData{
		HeroCostKnown: true, HeroCost: 65.59, HeroSaved: 100, HeroHitRate: 0.9,
		HeroCostExcluded: []string{"codex", "copilot"},
	}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboardHero(d, r, 99, i18n.EN, false)
	if !strings.Contains(out, "excl. codex, copilot") {
		t.Fatalf("renderDashboardHero() = %q, want the cost label to name the excluded agents (\"excl. codex, copilot\")", out)
	}
}

// TestRenderDashboardHeroOmitsExclusionLabelWhenEveryAgentIsPriced:
// the plain "cost" label stays unchanged when every agent in scope
// has a known price (no HeroCostExcluded).
func TestRenderDashboardHeroOmitsExclusionLabelWhenEveryAgentIsPriced(t *testing.T) {
	d := dashboardData{HeroCostKnown: true, HeroCost: 65.59, HeroSaved: 100, HeroHitRate: 0.9}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	out := renderDashboardHero(d, r, 99, i18n.EN, false)
	if strings.Contains(out, "excl.") {
		t.Fatalf("renderDashboardHero() = %q, want no exclusion label when every agent is priced", out)
	}
}

// TestRenderPanelMeasuredKeepsSessionCountPhraseTogetherWhenWrapping
// is the follow-up card's point D regression test: at a width that
// makes the codex row wrap, the "<not>'N sans usage'<newline>" count
// phrase must never be split mid-phrase (word-by-word wrapping broke
// it as "...26 sessions), 4" / "sans usage" on a real 200x50 fr
// screen) -- it must wrap on whole-segment boundaries instead, so
// the quantity phrase always stays on one line.
func TestRenderPanelMeasuredKeepsSessionCountPhraseTogetherWhenWrapping(t *testing.T) {
	d := dashboardData{Measured: []dashboardMeasuredRow{
		{
			Agent: "codex", Source: dashboardMeasuredSource[string(agents.Codex)],
			Period: i18n.T(i18n.FR, "card.last_days", 7), Sessions: 26, SessionsNoUsage: 4,
		},
	}}
	r := render.New(render.Options{IsTTY: true, NoColor: true})
	const width = 90
	lines := renderPanelMeasured(d, r, i18n.FR, width)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "4 sans usage") {
		t.Fatalf("renderPanelMeasured(width=%d) = %q, want the phrase \"4 sans usage\" to stay together on one line (wrap on segment boundaries, not mid-phrase)", width, joined)
	}
}
