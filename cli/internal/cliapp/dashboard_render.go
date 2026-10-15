package cliapp

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// dashboardHeroBlockDigitsMinWidth is the width (of the hero's own
// column -- see renderDashboardHero's width param, which is the
// column it is laid out in at every responsive tier, not the raw
// terminal width) at or above which the hero section's three key
// figures (cost, saved, hit rate) render as 3-line block digits inside
// tiles (card: ">=60 cols of the hero's own column, not the terminal").
// Narrower columns, and the non-interactive `ats dashboard` text path
// (allowBlockDigits=false), get a plain bold line instead.
const dashboardHeroBlockDigitsMinWidth = 60

// dashboardBody renders the Dashboard tab's full View() content for
// the interactive TUI: the hero summary box, the responsive 1-3 column
// grid (Taux de cache par agent / Par jour / Par modele), then the two
// full-width panels (A ameliorer, Mesure) -- built entirely from data
// already loaded by loadCmd (see buildDashboard): no second
// computation path.
func (m tuiModel) dashboardBody(width, height int) string {
	scope := ""
	if !m.allAgents {
		if kind, ok := m.currentAgent(); ok {
			scope = string(kind)
		}
	}
	d := buildDashboard(
		m.cacheResults, m.ttlResults, m.fixFinding, m.fixHasFinding,
		m.dailyHermes, m.dailyClaude, m.dailyHermesLong, m.now(), windowDays[m.dayIdx],
		m.hermesCompare, m.claudeCompare,
		m.skipped, scope, m.lang,
	)
	return renderDashboard(d, m.renderer(), width, height, m.lang, true)
}

// dashboardWideLayoutMinWidth / dashboardMediumLayoutMinWidth /
// dashboardActivityMinWidth gate the responsive layout tiers (card:
// ">=150 col" wide, "100-149 col" medium, "Activite hidden below 120",
// "<100: one column").
const (
	dashboardWideLayoutMinWidth   = 150
	dashboardMediumLayoutMinWidth = 100
	dashboardActivityMinWidth     = 120
)

// dashboardHistogramMinRows / dashboardHistogramDefaultRows /
// dashboardHistogramBoxOverhead size the "Depenses quotidiennes"
// vertical bar chart (card: "the histogram height absorbs the
// remaining terminal rows (min 5)"). BoxOverhead is the line count
// renderPanelHistogram/renderPanelBox always add besides the bars
// themselves (2 border lines + the chips header + the first/last day
// label) -- the wide (>=150) layout subtracts this, and every other
// row's own rendered height, from the real terminal height so the
// chart fills exactly what is left instead of leaving an empty bottom
// half. The medium/narrow tiers (where the chart shares rows with
// other panels of unrelated height, not one dedicated full-width row)
// keep the fixed default instead: there is no single "remaining space"
// left to divide once panels are packed 2-per-row or one-per-row.
const (
	dashboardHistogramMinRows     = 5
	dashboardHistogramDefaultRows = 8
	// dashboardHistogramMaxRows caps the bar chart's own height (card
	// point 4: "at 7 j it draws 7 giant bars eating half the screen
	// ... cap the histogram height (e.g. <= 12 rows)"): any height the
	// chart does not use because of this cap is handed to the
	// Activite/Taux par agent panels instead (renderDashboardWide),
	// never left as wasted blank rows above the footer.
	dashboardHistogramMaxRows     = 12
	dashboardHistogramBoxOverhead = 4
)

// renderDashboard renders dashboardData into the full dashboard view,
// picking one of three responsive layouts by width (card point 1).
// Used by both the interactive TUI (allowBlockDigits=true, height is
// the viewport's real remaining rows) and `ats dashboard`'s
// non-interactive text output (allowBlockDigits=false, height=0 since
// there is no fixed viewport to size the histogram against -- see
// dashboardHistogramRows). Colored() alone (via r) decides ANSI
// styling vs plain ASCII -- the layout itself never branches on color,
// so NO_COLOR stays byte-identical in structure to the colored run.
func renderDashboard(d dashboardData, r render.Renderer, width, height int, lang i18n.Lang, allowBlockDigits bool) string {
	switch {
	case width >= dashboardWideLayoutMinWidth:
		return renderDashboardWide(d, r, width, height, lang, allowBlockDigits)
	case width >= dashboardMediumLayoutMinWidth:
		return renderDashboardMedium(d, r, width, lang, allowBlockDigits)
	default:
		return renderDashboardNarrow(d, r, width, lang, allowBlockDigits)
	}
}

// renderDashboardWide is the >=150 col layout (card): row 1 hero +
// Efficacite, row 2 Activite + Taux de cache par agent, row 3 Depenses
// quotidiennes full width, row 4 A ameliorer + Par modele + Mesure.
func renderDashboardWide(d dashboardData, r render.Renderer, width, height int, lang i18n.Lang, allowBlockDigits bool) string {
	half := (width - 1) / 2
	third := (width - 2) / 3

	hero := renderDashboardHero(d, r, half, lang, allowBlockDigits)
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, hero, " ", dashboardEfficiencyBox(d, r, half, lang, allowBlockDigits))
	row4 := lipgloss.JoinHorizontal(lipgloss.Top,
		dashboardImproveBox(d, r, third, lang), " ",
		dashboardByModelBox(d, r, third, lang), " ",
		dashboardMeasuredBox(d, r, third, lang))

	// Activite shrinks to its fixed 12-week grid's own content width
	// (card point 5: "Activite heatmap uses ~1/4 of its panel width
	// ... shrink the panel to content; no big empty boxes"); the
	// recovered columns go to its row2 neighbor instead of sitting
	// mostly empty.
	activityWidth := dashboardActivityBoxWidth(half)
	byAgentWidth := width - 1 - activityWidth

	activityLines := renderPanelActivity(d, r, lang)
	byAgentLines := renderPanelByAgent(d, r, lang, byAgentWidth)
	row2Content := len(activityLines)
	if n := len(byAgentLines); n > row2Content {
		row2Content = n
	}

	// The lowered histogram cap (card point 4: "cap the histogram
	// height (e.g. <= 12 rows)") can free up height a tall terminal
	// would otherwise waste as blank rows above the footer; hand it to
	// row2's two panels instead (card: "give the freed rows to
	// Activite / Taux par agent panels") -- this also equalizes both
	// boxes to the same height, fixing the "agent panel is half empty"
	// look whenever its own natural content is shorter than Activite's.
	if height > 0 {
		fixedRows := countLines(row1) + 1 + countLines(row4)
		remaining := height - fixedRows - (row2Content + 2) - dashboardHistogramBoxOverhead
		if remaining > dashboardHistogramMaxRows {
			row2Content += remaining - dashboardHistogramMaxRows
		}
	}

	activityBox := renderPanelBox(r, i18n.T(lang, "dashboard.panel_activity"), padLines(activityLines, row2Content), activityWidth, dashboardPanelColor(r, 0))
	byAgentBox := renderPanelBox(r, i18n.T(lang, "dashboard.panel_by_agent"), padLines(byAgentLines, row2Content), byAgentWidth, dashboardPanelColor(r, 1))
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, activityBox, " ", byAgentBox)

	usedRows := countLines(row1) + 1 + countLines(row2) + countLines(row4)
	row3 := dashboardHistogramBox(d, r, width, lang, dashboardHistogramRows(height, usedRows))

	return row1 + "\n\n" + row2 + "\n" + row3 + "\n" + row4
}

// padLines appends blank lines to lines until it has at least n
// (never truncates: n smaller than len(lines) is a no-op), so two
// panels meant to sit side by side in the same row can be grown to a
// shared height.
func padLines(lines []string, n int) []string {
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines
}

// renderDashboardMedium is the 100-149 col layout (card: "same order,
// 2 per row; Activite hidden below 120").
func renderDashboardMedium(d dashboardData, r render.Renderer, width int, lang i18n.Lang, allowBlockDigits bool) string {
	colWidth := (width - 1) / 2
	items := []string{
		renderDashboardHero(d, r, colWidth, lang, allowBlockDigits),
		dashboardEfficiencyBox(d, r, colWidth, lang, allowBlockDigits),
	}
	if width >= dashboardActivityMinWidth {
		items = append(items, dashboardActivityBox(d, r, colWidth, lang))
	}
	items = append(items,
		dashboardByAgentBox(d, r, colWidth, lang),
		dashboardHistogramBox(d, r, colWidth, lang, dashboardHistogramDefaultRows),
		dashboardImproveBox(d, r, colWidth, lang),
		dashboardByModelBox(d, r, colWidth, lang),
		dashboardMeasuredBox(d, r, colWidth, lang),
	)
	return packDashboardItems(items, 2)
}

// renderDashboardNarrow is the <100 col layout (card: "one column").
func renderDashboardNarrow(d dashboardData, r render.Renderer, width int, lang i18n.Lang, allowBlockDigits bool) string {
	items := []string{
		renderDashboardHero(d, r, width, lang, allowBlockDigits),
		dashboardEfficiencyBox(d, r, width, lang, allowBlockDigits),
		dashboardByAgentBox(d, r, width, lang),
		dashboardHistogramBox(d, r, width, lang, dashboardHistogramDefaultRows),
		dashboardImproveBox(d, r, width, lang),
		dashboardByModelBox(d, r, width, lang),
		dashboardMeasuredBox(d, r, width, lang),
	}
	return packDashboardItems(items, 1)
}

// packDashboardItems arranges items perRow-per-row, each row's items
// joined horizontally with a single-space gutter, rows stacked with a
// newline (card: "100-149 col: ... 2 per row"; "<100: one column").
func packDashboardItems(items []string, perRow int) string {
	var rows []string
	for i := 0; i < len(items); i += perRow {
		end := i + perRow
		if end > len(items) {
			end = len(items)
		}
		row := items[i]
		for _, it := range items[i+1 : end] {
			row = lipgloss.JoinHorizontal(lipgloss.Top, row, " ", it)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

// countLines reports how many lines s renders as.
func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

// dashboardHistogramRows sizes the "Depenses quotidiennes" bar chart:
// height<=0 means no real terminal height is known (the non-interactive
// `ats dashboard` text path), so it keeps the fixed default; otherwise
// it absorbs whatever height remains after usedRows (every other row
// already rendered) and the chart's own border/chips/label overhead,
// floored at dashboardHistogramMinRows so it can never collapse to
// nothing on a very short terminal.
func dashboardHistogramRows(height, usedRows int) int {
	if height <= 0 {
		return dashboardHistogramDefaultRows
	}
	remaining := height - usedRows - dashboardHistogramBoxOverhead
	if remaining < dashboardHistogramMinRows {
		remaining = dashboardHistogramMinRows
	}
	if remaining > dashboardHistogramMaxRows {
		remaining = dashboardHistogramMaxRows
	}
	return remaining
}

// dashboardEfficiencyBox / dashboardActivityBox / dashboardByAgentBox /
// dashboardHistogramBox / dashboardImproveBox / dashboardByModelBox /
// dashboardMeasuredBox each wrap one panel's already-tested content
// builder (renderPanelX) in its bordered box at the given width, so
// every responsive tier (wide/medium/narrow) can place the same panel
// at a different width without duplicating renderPanelBox's styling.
func dashboardEfficiencyBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang, allowBlockDigits bool) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_efficiency"), renderPanelEfficiency(d, r, lang, allowBlockDigits), width, dashboardPanelColor(r, 2))
}

func dashboardActivityBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_activity"), renderPanelActivity(d, r, lang), width, dashboardPanelColor(r, 0))
}

func dashboardByAgentBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_by_agent"), renderPanelByAgent(d, r, lang, width), width, dashboardPanelColor(r, 1))
}

func dashboardHistogramBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang, rows int) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_histogram"), renderPanelHistogram(d, r, lang, rows, width), width, dashboardPanelColor(r, 3))
}

func dashboardImproveBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_improve"), renderPanelImprove(d, r, lang, width), width, dashboardPanelColor(r, 4))
}

func dashboardByModelBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_by_model"), renderPanelByModel(d, r, lang, width), width, dashboardPanelColor(r, 0))
}

func dashboardMeasuredBox(d dashboardData, r render.Renderer, width int, lang i18n.Lang) string {
	return renderPanelBox(r, i18n.T(lang, "dashboard.panel_measured"), renderPanelMeasured(d, r, lang, width), width, dashboardPanelColor(r, 1))
}

// renderDashboardHero renders panel A: the big headline figures
// (cost, saved, hit rate -- plus calls/sessions in the plain-line
// fallback), the dim token-counts line, and the optional
// previous-period comparison line.
func renderDashboardHero(d dashboardData, r render.Renderer, width int, lang i18n.Lang, allowBlockDigits bool) string {
	costStr := i18n.T(lang, "dashboard.unknown_price_mark")
	savedStr := costStr
	if d.HeroCostKnown {
		costStr = render.FormatUSDLang(d.HeroCost, lang)
		savedStr = render.FormatUSDLang(d.HeroSaved, lang)
	}
	hitStr := render.FormatPercentLang(d.HeroHitRate*100, lang, 1)

	var sb strings.Builder
	if allowBlockDigits && width >= dashboardHeroBlockDigitsMinWidth {
		accent, _, _ := r.ChromeColors()
		tileWidth := (width - 2) / 3
		tiles := []string{
			renderHeroTile(costStr, i18n.T(lang, "dashboard.hero_cost"), r, tileWidth, accent),
			renderHeroTile(savedStr, i18n.T(lang, "dashboard.hero_saved"), r, tileWidth, r.SemanticColor(render.SemanticGood)),
			renderHeroTile(hitStr, i18n.T(lang, "dashboard.hero_hit_rate"), r, tileWidth, r.SemanticColor(render.HitRateSemantic(d.HeroHitRate))),
		}
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tiles[0], " ", tiles[1], " ", tiles[2]))
	} else {
		line := fmt.Sprintf("%s %s   %s %s   %s %s",
			costStr, i18n.T(lang, "dashboard.hero_cost"),
			savedStr, i18n.T(lang, "dashboard.hero_saved"),
			hitStr, i18n.T(lang, "dashboard.hero_hit_rate"))
		if r.Colored() {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		sb.WriteString(line)
	}
	sb.WriteString("\n")
	for _, l := range renderHeroMiniStats(d, r, lang) {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	tokLine := i18n.T(lang, "dashboard.hero_tokens_line",
		render.FormatCountLang(d.TokRead, lang), render.FormatCountLang(d.TokWrite, lang),
		render.FormatCountLang(d.TokInput, lang), render.FormatCountLang(d.TokOutput, lang))
	sb.WriteString(r.Muted(tokLine))
	if d.CompareLine != "" {
		sb.WriteString("\n")
		sb.WriteString(r.Muted(d.CompareLine))
	}
	return truncateLinesToWidth(sb.String(), width)
}

// truncateLinesToWidth truncates (ansi.Truncate, ANSI- and wide-rune
// aware) every line of s to at most width visible columns, ellipsis on
// cut: the hero section is not boxed by renderPanelBox, so without
// this its free-form text lines (the plain-fallback headline, the dim
// token-counts line, mini-stats, the period-comparison line) have no
// other width guard -- harmless while hero got the full terminal
// width, but a real overflow once the wide/medium layouts confine it
// to half a column or less next to Efficacite.
func truncateLinesToWidth(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "\u2026")
	}
	return strings.Join(lines, "\n")
}

// renderHeroMiniStats builds the hero section's mini-stats lines (card
// 1a): calls/sessions, then yesterday/7-day-average/vs-yesterday swing
// -- colored red when cost rose, green when it fell or held steady.
// Each *Known flag (see buildHeroMiniStats) independently gates its own
// segment, so an agent with no daily history at all (codex, copilot,
// gemini-cli) just gets the calls/sessions line, never an invented
// trend below it.
func renderHeroMiniStats(d dashboardData, r render.Renderer, lang i18n.Lang) []string {
	lines := []string{r.Muted(i18n.T(lang, "dashboard.mini_calls_sessions",
		render.FormatCountLang(d.HeroCalls, lang), render.FormatCountLang(d.HeroSessions, lang)))}

	var parts []string
	if d.MiniYesterdayKnown {
		parts = append(parts, i18n.T(lang, "dashboard.mini_yesterday", render.FormatUSDLang(d.MiniYesterday, lang)))
	}
	if d.MiniAvg7Known {
		parts = append(parts, i18n.T(lang, "dashboard.mini_avg7", render.FormatUSDLang(d.MiniAvg7, lang)))
	}
	if d.MiniVsYesterdayKnown {
		swing := i18n.T(lang, "dashboard.mini_vs_yesterday", formatSignedPercentLang(d.MiniVsYesterdayPct, lang))
		if r.Colored() {
			sem := render.SemanticGood
			if d.MiniVsYesterdayPct > 0 {
				sem = render.SemanticBad
			}
			swing = lipgloss.NewStyle().Foreground(r.SemanticColor(sem)).Render(swing)
		}
		parts = append(parts, swing)
	}
	if len(parts) > 0 {
		lines = append(lines, strings.Join(parts, "   "))
	}
	return lines
}

// formatSignedPercentLang is render.FormatPercentLang (0 decimals) with
// an explicit "+" prefix for a positive value (card: "vs hier +19 %");
// a negative value already carries its own "-" sign from FormatFloat.
func formatSignedPercentLang(v float64, lang i18n.Lang) string {
	s := render.FormatPercentLang(v, lang, 0)
	if v > 0 {
		return "+" + s
	}
	return s
}

// renderHeroTile renders one hero figure as a 3-row block-digit banner
// (render.BlockDigits) over its plain-words label, both padded to
// width so the three tiles line up in a row.
func renderHeroTile(value, label string, r render.Renderer, width int, c color.Color) string {
	var sb strings.Builder
	for _, row := range render.BlockDigits(value) {
		styled := row
		if r.Colored() {
			styled = lipgloss.NewStyle().Bold(true).Foreground(c).Render(row)
		}
		sb.WriteString(lipgloss.NewStyle().Width(width).Render(styled))
		sb.WriteString("\n")
	}
	labelStyled := label
	if r.Colored() {
		_, labelColor, _ := r.ChromeColors()
		labelStyled = lipgloss.NewStyle().Foreground(labelColor).Render(label)
	}
	sb.WriteString(lipgloss.NewStyle().Width(width).Render(labelStyled))
	return sb.String()
}

// heatmapCellWidth is each Activite cell's visible column width (card
// point 3: "Each cell = '[U+25A0] ' (2 cols)"): the shaded glyph plus
// one trailing blank column, so cells read as distinct squares
// instead of a run-on wall of blocks.
const heatmapCellWidth = 2

// heatmapGutterWidth is the left margin reserved for the day-of-week
// labels (card: "labels lun/mer/ven on the left"): the longest
// abbreviation (3 letters) plus one separating space.
const heatmapGutterWidth = 4

// heatmapDayAbbrev are the day-of-week labels shown on the grid's left
// margin, localized -- only every other row (Monday/Wednesday/Friday)
// carries one, enough to orient without labelling all 7 rows.
var heatmapDayAbbrev = map[i18n.Lang][7]string{
	i18n.EN: {"Mon", "", "Wed", "", "Fri", "", ""},
	i18n.FR: {"lun", "", "mer", "", "ven", "", ""},
}

// dashboardActivityBoxWidth sizes the "Activite" panel's own box
// width to its fixed 12-week grid's content (card point 5: "Activite
// heatmap uses ~1/4 of its panel width ... shrink the panel to
// content; no big empty boxes"), never more than half (its fair
// share of row2's width, when that is narrower than the grid itself).
// The recovered columns go to its row2 neighbor (Taux de cache par
// agent) in renderDashboardWide.
func dashboardActivityBoxWidth(half int) int {
	content := heatmapGutterWidth + dashboardHeatmapWeeks*heatmapCellWidth + 4
	if content > half {
		return half
	}
	return content
}

// buildHeatmapMonthLine places each non-zero monthStarts[w] label at
// its own week column (w*heatmapCellWidth), skipping any label whose
// starting column would fall before the previous label's own end
// column (card: "skipped if it would overlap the previous one") --
// the root fix for the "juilaout" collision bug, which came from the
// old renderer writing a multi-rune label into what it treated as a
// single-column slot and letting it bleed into the next column's text
// instead of tracking real columns at all.
func buildHeatmapMonthLine(monthStarts []time.Month, months [12]string) string {
	total := len(monthStarts) * heatmapCellWidth
	buf := []rune(strings.Repeat(" ", total))
	nextFree := 0
	for w, m := range monthStarts {
		if m == 0 {
			continue
		}
		label := []rune(months[int(m)-1])
		start := w * heatmapCellWidth
		if start < nextFree {
			continue
		}
		end := start + len(label)
		if end > total {
			end = total
			label = label[:end-start]
		}
		copy(buf[start:end], label)
		nextFree = end + 1 // a 1-column gap before the next label
	}
	return string(buf)
}

// renderPanelActivity builds panel "Activite"'s rows: a month-label
// line, a 7-row (Mon..Sun) by dashboardHeatmapWeeks-column grid with
// day-of-week labels on the left (each cell shaded by heatmapShade:
// 4 orange shades, or the dim "no data" color for days before the
// agent's own history or after `now` -- see buildActivityHeatmap),
// and a trailing "N active days" summary line.
func renderPanelActivity(d dashboardData, r render.Renderer, lang i18n.Lang) []string {
	h := d.Heatmap
	if len(h.Weeks) == 0 {
		return []string{i18n.T(lang, "dashboard.no_daily_data")}
	}
	months := HeatmapMonthAbbrev[lang]
	monthLine := buildHeatmapMonthLine(h.MonthStarts, months)
	lines := []string{r.Muted(strings.Repeat(" ", heatmapGutterWidth) + monthLine)}

	dayAbbrev := heatmapDayAbbrev[lang]
	for day := 0; day < 7; day++ {
		var row strings.Builder
		row.WriteString(r.Muted(fixedWidth(dayAbbrev[day], heatmapGutterWidth, false)))
		for w := range h.Weeks {
			row.WriteString(heatmapGlyph(r, heatmapShade(h.Weeks[w][day].Cost, h.MaxCost)))
		}
		lines = append(lines, row.String())
	}
	lines = append(lines, r.Muted(i18n.T(lang, "dashboard.heatmap_active_days", h.ActiveDays)))
	return lines
}

// heatmapGlyph renders one Activite cell (heatmapCellWidth columns
// wide): a shaded square colored by heatmapCellColor on a color TTY,
// or a plain ASCII density character otherwise (card: "4 orange
// shades by daily cost + empty dim cell"), followed by one blank
// column so adjacent cells read as distinct squares.
func heatmapGlyph(r render.Renderer, shade int) string {
	if !r.Colored() {
		asciiGlyphs := []string{".", "-", "+", "*", "#"} // index 0 = no data (shade -1)
		idx := shade + 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(asciiGlyphs) {
			idx = len(asciiGlyphs) - 1
		}
		return asciiGlyphs[idx] + " "
	}
	return lipgloss.NewStyle().Foreground(heatmapCellColor(r, shade)).Render("\u25a0") + " "
}

// heatmapCellColor is the Activite heatmap's own fixed 4-shade orange
// gradient (lightest to darkest), or the renderer's dim chrome color
// for an empty cell (shade -1, see heatmapShade) -- a small palette
// local to this one panel, since neither CostBucketColor nor
// SemanticColor fits a 4-step magnitude gradient.
func heatmapCellColor(r render.Renderer, shade int) color.Color {
	if shade < 0 {
		_, _, dim := r.ChromeColors()
		return dim
	}
	shades := []color.Color{
		lipgloss.Color("#f3b58c"), lipgloss.Color("#e8905a"),
		lipgloss.Color("#d97757"), lipgloss.Color("#a84a2e"),
	}
	if shade >= len(shades) {
		shade = len(shades) - 1
	}
	return shades[shade]
}

// renderPanelHistogram builds panel "Depenses quotidiennes"'s rows:
// the headline chips (Moy./jour, Pic + its date, Hier), the vertical
// bar chart spread across the panel's full width (card point 4: bar
// width = max(1, (panelWidth-axis)/days) with a 1-col gap, a $ axis
// on the right at 0/mid/max, a dotted reference line at the peak
// level, and first/middle/last day labels under their own bar).
func renderPanelHistogram(d dashboardData, r render.Renderer, lang i18n.Lang, rows, width int) []string {
	h := d.Histogram
	if len(h.Bars) == 0 {
		return []string{i18n.T(lang, "dashboard.no_daily_data")}
	}

	peak := render.FormatUSDLang(h.Peak, lang)
	if h.PeakDay != "" {
		peak += " " + render.FormatShortDateLang(h.PeakDay, lang)
	}
	avgChip := fmt.Sprintf("%s %s", i18n.T(lang, "dashboard.histogram_avg"), render.FormatUSDLang(h.AvgPerDay, lang))
	peakChip := fmt.Sprintf("%s %s", i18n.T(lang, "dashboard.histogram_peak"), peak)
	yesterdayChip := fmt.Sprintf("%s %s", i18n.T(lang, "dashboard.histogram_yesterday"), render.FormatUSDLang(h.Yesterday, lang))

	// chipsInner is the box's own inner width (renderPanelBox: width-4),
	// computed independently of the bars-area `inner` floor below so a
	// narrow panel never lets the generic box-level ansi.Truncate cut
	// into a money figure (card point 6: "never truncate a money or %
	// value"). Fields are dropped whole, widest/least essential first
	// (Hier, then the peak date suffix), never truncated mid-value.
	chipsInner := width - 4
	chips := avgChip + "   " + peakChip + "   " + yesterdayChip
	if len([]rune(chips)) > chipsInner {
		chips = avgChip + "   " + peakChip
	}
	if len([]rune(chips)) > chipsInner {
		chips = avgChip + "   " + fmt.Sprintf("%s %s", i18n.T(lang, "dashboard.histogram_peak"), render.FormatUSDLang(h.Peak, lang))
	}
	lines := []string{r.Muted(chips)}

	days := len(h.Bars)
	inner := width - 4
	if inner < days+10 {
		inner = days + 10
	}
	maxLabel := render.FormatUSDLang(h.Peak, lang)
	midLabel := render.FormatUSDLang(h.Peak/2, lang)
	zeroLabel := render.FormatUSDLang(0, lang)
	axisWidth := len([]rune(maxLabel))
	for _, l := range []string{midLabel, zeroLabel} {
		if w := len([]rune(l)); w > axisWidth {
			axisWidth = w
		}
	}
	axisWidth++ // 1-column gap before the axis text
	barsAreaWidth := inner - axisWidth
	if barsAreaWidth < days {
		barsAreaWidth = days
	}
	barWidth := (barsAreaWidth - (days - 1)) / days
	if barWidth < 1 {
		barWidth = 1
	}
	usedCols := barWidth*days + (days - 1)
	if usedCols > barsAreaWidth {
		barsAreaWidth = usedCols
	}

	// Expand one value per day into barWidth duplicate columns
	// separated by a 1-column gap (a 0, which VerticalBars always
	// leaves blank): this reuses VerticalBars' own per-column
	// proportional-height logic instead of a second chart engine.
	expanded := make([]float64, 0, usedCols)
	for i, b := range h.Bars {
		for c := 0; c < barWidth; c++ {
			expanded = append(expanded, b.Cost)
		}
		if i < days-1 {
			expanded = append(expanded, 0)
		}
	}

	barColor := r.CostBucketColor("cache_write")
	axisLabels := map[int]string{0: maxLabel, rows / 2: midLabel, rows - 1: zeroLabel}
	for rowIdx, barRow := range render.VerticalBars(expanded, rows) {
		if rowIdx == 0 {
			// Dotted reference line at the peak level (card: "dotted
			// line at peak"): any column that is not already a bar
			// glyph gets a dim dot instead of staying blank, so the
			// peak threshold reads across the whole chart width.
			runes := []rune(barRow)
			for i, c := range runes {
				if c == ' ' {
					runes[i] = '.'
				}
			}
			barRow = string(runes)
		}
		padded := lipgloss.NewStyle().Width(barsAreaWidth).Render(barRow)
		styled := padded
		if r.Colored() {
			styled = lipgloss.NewStyle().Foreground(barColor).Render(padded)
		}
		axisText := axisLabels[rowIdx]
		lines = append(lines, styled+" "+fixedWidth(axisText, axisWidth-1, true))
	}

	// First/middle/last day labels centered under their own bar.
	labelLine := []rune(strings.Repeat(" ", barsAreaWidth))
	placeLabel := func(dayIdx int, text string) {
		start := dayIdx * (barWidth + 1)
		col := start + barWidth/2 - len([]rune(text))/2
		if col < 0 {
			col = 0
		}
		if col >= len(labelLine) {
			return
		}
		end := col + len([]rune(text))
		if end > len(labelLine) {
			end = len(labelLine)
			text = string([]rune(text)[:end-col])
		}
		copy(labelLine[col:end], []rune(text))
	}
	placeLabel(0, render.FormatShortDateLang(h.Bars[0].Day, lang))
	placeLabel(days/2, render.FormatShortDateLang(h.Bars[days/2].Day, lang))
	placeLabel(days-1, render.FormatShortDateLang(h.Bars[days-1].Day, lang))
	lines = append(lines, string(labelLine))
	return lines
}

// dashboardPanelColor assigns each panel (by index, 0-4) a distinct,
// fixed border/title color (card: "each panel its own border color"),
// reusing the renderer's existing color helpers instead of inventing a
// new palette.
func dashboardPanelColor(r render.Renderer, idx int) color.Color {
	switch idx % 5 {
	case 0:
		return r.CostBucketColor("cache_read")
	case 1:
		return r.CostBucketColor("input")
	case 2:
		return r.CostBucketColor("output")
	case 3:
		return r.SemanticColor(render.SemanticWarn)
	default:
		return r.SemanticColor(render.SemanticBad)
	}
}

// renderPanelBox draws body inside a rounded box with "<title>"
// embedded in the top border, colored borderColor. Box-drawing
// characters are always kept (same rule render.Table already follows):
// only the ANSI color styling is gated by r.Colored(), so a NO_COLOR
// or non-TTY run still gets the same bordered structure in plain
// ASCII-safe runes. Each line is truncated (ansi.Truncate, ANSI- and
// wide-rune aware) to the box's inner width before padding: lipgloss's
// own Width() wraps rather than truncates oversized content (the exact
// bug a previous reviewer already hit on the TTL panel, and the one a
// narrower row4 column exposed again on the Measured panel's longer
// lines), so a line that does not fit never breaks out past the box's
// right border onto an extra visual row.
func renderPanelBox(r render.Renderer, title string, lines []string, width int, borderColor color.Color) string {
	if width < 30 {
		width = 30
	}
	b := lipgloss.RoundedBorder()
	colored := r.Colored()
	style := func(s string, bold bool) string {
		if !colored {
			return s
		}
		st := lipgloss.NewStyle().Foreground(borderColor)
		if bold {
			st = st.Bold(true)
		}
		return st.Render(s)
	}

	titleLen := len([]rune(title))
	dashesAfter := width - 5 - titleLen
	if dashesAfter < 1 {
		dashesAfter = 1
	}
	var sb strings.Builder
	sb.WriteString(style(b.TopLeft+b.Top+" ", false))
	sb.WriteString(style(title, true))
	sb.WriteString(style(" "+strings.Repeat(b.Top, dashesAfter)+b.TopRight, false))
	sb.WriteString("\n")

	innerWidth := width - 4
	if len(lines) == 0 {
		lines = []string{""}
	}
	for _, line := range lines {
		fitted := ansi.Truncate(line, innerWidth, "\u2026")
		padded := lipgloss.NewStyle().Width(innerWidth).Render(fitted)
		sb.WriteString(style(b.Left, false))
		sb.WriteString(" ")
		sb.WriteString(padded)
		sb.WriteString(" ")
		sb.WriteString(style(b.Right, false))
		sb.WriteString("\n")
	}
	sb.WriteString(style(b.BottomLeft+strings.Repeat(b.Bottom, width-2)+b.BottomRight, false))
	return sb.String()
}

// truncateRunes hard-truncates s to at most w runes, appending an
// ellipsis when it had to cut (e.g. a long model name), so a column
// never receives content wider than its fixedWidth call -- lipgloss's
// Width() wraps rather than truncates oversized content (the exact bug
// a previous reviewer already hit on the TTL panel), so every variable
// -length cell is pre-truncated here before any fixedWidth call.
func truncateRunes(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:w])
	}
	return string(r[:w-1]) + "\u2026"
}

// fixedWidth renders s padded/truncated to exactly w visible columns,
// left- or right-aligned: the building block every compact panel row
// uses so columns line up regardless of how long any one cell's text
// is (card: "right-aligned numeric columns").
func fixedWidth(s string, w int, right bool) string {
	st := lipgloss.NewStyle().Width(w).MaxWidth(w)
	if right {
		st = st.Align(lipgloss.Right)
	}
	return st.Render(s)
}

// dashboardMagnitudeColor buckets frac (a value normalized against its
// series' own max, in [0,1]) into the card's "blue -> yellow -> orange
// -> red by magnitude" gradient, reusing the renderer's existing
// semantic/cost-bucket colors instead of inventing a new palette.
func dashboardMagnitudeColor(r render.Renderer, frac float64) color.Color {
	switch {
	case frac >= 0.75:
		return r.SemanticColor(render.SemanticBad)
	case frac >= 0.4:
		return r.SemanticColor(render.SemanticWarn)
	default:
		return r.CostBucketColor("input")
	}
}

// renderPanelByAgent builds panel B's rows: one per detected agent,
// a hit-rate bar, hit %, call count and API-equivalent cost. Unknown
// price renders "-" plus a dim footnote (card: "Unknown price -> '-'
// and a dim footnote"). width is the panel's own outer box width (as
// passed to renderPanelBox): at a narrow width the bar (purely
// decorative) is dropped first, then the calls count, before the
// agent name column itself shrinks -- the cost/saved ($) and hit-rate
// (%) columns are never shrunk or truncated (card point 6).
func renderPanelByAgent(d dashboardData, r render.Renderer, lang i18n.Lang, width int) []string {
	inner := width - 4
	if inner < 20 {
		inner = 20
	}
	agentW, barW, hitW, callsW := 12, 7, 6, 6
	// costW/savedW are sized from the widest formatted value actually
	// present (card point 1: "size columns from the widest formatted
	// value; never wrap a row"), floored at the header labels' own
	// width -- a 4-digit cost (e.g. "1812,36 $", 9 cols) must never be
	// squeezed into a fixed 8-column field, which made lipgloss wrap
	// the row onto a second visual line and misplace the right border.
	costW, savedW := 8, 10
	for _, row := range d.AgentRows {
		if !row.CostKnown {
			continue
		}
		if w := len([]rune(render.FormatUSDLang(row.Cost, lang))); w > costW {
			costW = w
		}
		if w := len([]rune(render.FormatUSDLang(row.Saved, lang))); w > savedW {
			savedW = w
		}
	}
	showBar, showCalls := true, true
	total := func() int {
		n, cols := hitW+costW+savedW+agentW, 4
		if showBar {
			n += barW
			cols++
		}
		if showCalls {
			n += callsW
			cols++
		}
		return n + cols - 1
	}
	if total() > inner {
		showBar = false
	}
	if total() > inner {
		showCalls = false
	}
	for total() > inner && agentW > 6 {
		agentW--
	}

	var lines []string
	if len(d.AgentRows) > 0 {
		cacheHeaderW := hitW
		if showBar {
			cacheHeaderW = barW + 1 + hitW
		}
		header := fixedWidth(i18n.T(lang, "dashboard.col_agent"), agentW, false) + " " +
			fixedWidth(i18n.T(lang, "dashboard.col_cache"), cacheHeaderW, false) + " "
		if showCalls {
			header += fixedWidth(i18n.T(lang, "dashboard.col_calls"), callsW, true) + " "
		}
		header += fixedWidth(i18n.T(lang, "dashboard.col_cost"), costW, true) + " " +
			fixedWidth(i18n.T(lang, "dashboard.col_saved"), savedW, true)
		lines = append(lines, r.Muted(header))
	}
	for _, row := range d.AgentRows {
		cost := i18n.T(lang, "dashboard.unknown_price_mark")
		saved := cost
		if row.CostKnown {
			cost = render.FormatUSDLang(row.Cost, lang)
			saved = render.FormatUSDLang(row.Saved, lang)
		}
		line := fixedWidth(truncateRunes(row.Agent, agentW), agentW, false) + " "
		if showBar {
			line += r.Bar(row.HitRate, barW, r.SemanticColor(render.HitRateSemantic(row.HitRate))) + " "
		}
		line += fixedWidth(render.FormatPercentLang(row.HitRate*100, lang, 0), hitW, true) + " "
		if showCalls {
			line += fixedWidth(render.FormatCountLang(row.Calls, lang), callsW, true) + " "
		}
		line += fixedWidth(cost, costW, true) + " " + fixedWidth(saved, savedW, true)
		lines = append(lines, line)
	}
	if d.AnyUnknownPrice {
		lines = append(lines, r.Muted(i18n.T(lang, "dashboard.unknown_price_footnote")))
	}
	if len(lines) == 0 {
		lines = []string{i18n.T(lang, "dashboard.no_agents")}
	}
	return lines
}

// renderPanelByDay builds panel C's rows: the last N days (newest
// first), a cost bar (gradient-colored by relative magnitude), cost
// and hit rate -- from the same per-day series the Cache panel's
// sparkline already uses, never a second parser.
func renderPanelByDay(d dashboardData, r render.Renderer, lang i18n.Lang) []string {
	if d.NoDailyNote != "" {
		return []string{r.Muted(d.NoDailyNote)}
	}
	var maxCost float64
	for _, row := range d.DayRows {
		if row.Cost > maxCost {
			maxCost = row.Cost
		}
	}
	var lines []string
	for _, row := range d.DayRows {
		frac := 0.0
		if maxCost > 0 {
			frac = row.Cost / maxCost
		}
		bar := r.Bar(frac, 12, dashboardMagnitudeColor(r, frac))
		lines = append(lines,
			fixedWidth(render.FormatShortDateLang(row.Day, lang), 5, false)+" "+bar+" "+
				fixedWidth(render.FormatUSDLang(row.Cost, lang), 8, true)+" "+
				fixedWidth(render.FormatPercentLang(row.HitRate*100, lang, 0), 5, true))
	}
	return lines
}

// renderPanelByModel builds panel D's rows (Copilot's per-model
// breakdown today; any other agent that ever reports ModelBreakdown
// joins automatically, nothing agent-specific is hardcoded here).
// Empty when no agent in scope reports per-model data -- never
// invented. width is the panel's own outer box width (as passed to
// renderPanelBox): the model-name column is sized from whatever space
// is left after the fixed hit-rate/calls columns, truncating only
// when the panel genuinely lacks room (card point 2: names were
// truncated at a hardcoded 12 columns even with ~60 free columns at
// 200x50).
func renderPanelByModel(d dashboardData, r render.Renderer, lang i18n.Lang, width int) []string {
	if d.ModelNote != "" {
		return []string{r.Muted(d.ModelNote)}
	}
	inner := width - 4
	if inner < 20 {
		inner = 20
	}
	hitW, callsW := 6, 6
	modelW := inner - 1 - hitW - 1 - callsW
	if modelW < 12 {
		modelW = 12
	}
	var lines []string
	for _, row := range d.ModelRows {
		lines = append(lines,
			fixedWidth(truncateRunes(row.Model, modelW), modelW, false)+" "+
				fixedWidth(render.FormatPercentLang(row.HitRate*100, lang, 0), hitW, true)+" "+
				fixedWidth(render.FormatCountLang(row.Calls, lang), callsW, true))
	}
	return lines
}

// renderPanelImprove builds panel E's rows: the ranked findings
// (buildFindings/rankFindings), each a short label, its estimated $ or
// token impact (when computed), and the key that opens the matching
// detail tab. Empty -> one line reading "nothing to fix" (card:
// "Empty -> one green line"). width is the panel's own outer box
// width (as passed to renderPanelBox): the amount column is always
// sized to the longest amount actually present (never truncated --
// card point 6), the label shrinks to whatever is left, and the
// "[N]" detail-key suffix is the first thing dropped when space runs
// out, so a money/token value is never the one that gets cut.
func renderPanelImprove(d dashboardData, r render.Renderer, lang i18n.Lang, width int) []string {
	if len(d.Findings) == 0 {
		line := i18n.T(lang, "dashboard.nothing_to_fix")
		if r.Colored() {
			line = lipgloss.NewStyle().Bold(true).Foreground(r.SemanticColor(render.SemanticGood)).Render(line)
		}
		return []string{line}
	}
	inner := width - 4
	if inner < 20 {
		inner = 20
	}
	amountWidth := 8
	for _, f := range d.Findings {
		if w := len([]rune(f.Amount)); w > amountWidth {
			amountWidth = w
		}
	}
	labelWidth := inner - 1 - amountWidth
	showKey := labelWidth-4 >= 10
	if showKey {
		labelWidth -= 4 // " [N]"
	}
	if labelWidth < 8 {
		labelWidth = 8
	}
	var lines []string
	for _, f := range d.Findings {
		line := fixedWidth(truncateRunes(f.Label, labelWidth), labelWidth, false) + " " + fixedWidth(f.Amount, amountWidth, true)
		if showKey && f.DetailKey != "" {
			line += " [" + f.DetailKey + "]"
		}
		lines = append(lines, line)
	}
	return lines
}

// renderPanelEfficiency builds panel "Efficacite"'s rows: the 0-100
// score as a big block-digit banner with its letter grade next to it
// (card point 5: "render the score big (block digits) + letter, bars
// right next to it, no blank rows"), then three labelled bars for the
// metrics that feed the score (buildEfficiency) -- cache hit rate,
// share of calls without an abnormal rebuild, and whether the TTL is
// already well tuned (shown as a 100%/0% bar, oui/non, never a second
// number the score itself did not already use).
func renderPanelEfficiency(d dashboardData, r render.Renderer, lang i18n.Lang, allowBlockDigits bool) []string {
	e := d.Efficiency
	sem := efficiencyScoreSemantic(e.Score)
	scoreColor := r.SemanticColor(sem)

	ttlWord := i18n.T(lang, "dashboard.no")
	ttlPct := 0.0
	if e.TTLWellTuned {
		ttlWord = i18n.T(lang, "dashboard.yes")
		ttlPct = 100
	}
	bar := func(label string, pct float64, s render.Semantic) string {
		return fixedWidth(truncateRunes(label, 28), 28, false) + " " +
			r.Bar(pct/100, 12, r.SemanticColor(s)) + " " +
			fixedWidth(render.FormatPercentLang(pct, lang, 0), 6, true)
	}
	bars := []string{
		bar(i18n.T(lang, "dashboard.eff_cache_rate"), e.CacheRatePct, render.HitRateSemantic(e.CacheRatePct/100)),
		bar(i18n.T(lang, "dashboard.eff_no_rebuild"), e.NoRebuildPct, render.HitRateSemantic(e.NoRebuildPct/100)),
		fixedWidth(truncateRunes(i18n.T(lang, "dashboard.eff_ttl_tuned"), 28), 28, false) + " " +
			r.Bar(ttlPct/100, 12, r.SemanticColor(efficiencyBoolSemantic(e.TTLWellTuned))) + " " +
			fixedWidth(ttlWord, 6, true),
	}

	// Block digits are an interactive-TUI-only flourish (same rule the
	// hero's own tiles follow): the non-interactive `ats dashboard`
	// text path always gets the plain score line instead.
	if !allowBlockDigits {
		scoreLine := i18n.T(lang, "dashboard.eff_score_line", e.Score, e.Letter)
		if r.Colored() {
			scoreLine = lipgloss.NewStyle().Bold(true).Foreground(scoreColor).Render(scoreLine)
		}
		return append([]string{scoreLine}, bars...)
	}

	scoreRows := render.BlockDigits(fmt.Sprintf("%d", e.Score))
	letter := e.Letter
	if r.Colored() {
		letter = lipgloss.NewStyle().Bold(true).Foreground(scoreColor).Render(letter)
	}
	// The letter grade sits beside the block digits' middle row, so
	// the 3-row score block reads "<digits> <letter>" on one visual
	// line instead of needing a 4th row of its own.
	scoreRows[1] += "  " + letter
	scoreWidth := 0
	for _, row := range scoreRows {
		if w := lipgloss.Width(row); w > scoreWidth {
			scoreWidth = w
		}
	}
	for i, row := range scoreRows {
		styled := row
		if r.Colored() {
			styled = lipgloss.NewStyle().Bold(true).Foreground(scoreColor).Render(row)
		}
		scoreRows[i] = lipgloss.NewStyle().Width(scoreWidth).Render(styled)
	}

	joined := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(scoreRows, "\n"), "  ", strings.Join(bars, "\n"))
	return strings.Split(joined, "\n")
}

// efficiencyScoreSemantic buckets the 0-100 score itself into the
// same good/warn/bad traffic-light HitRateSemantic already uses
// (>=85 good, >=60 warn, below bad), so the headline reads at a
// glance without a fourth color scale.
func efficiencyScoreSemantic(score int) render.Semantic {
	return render.HitRateSemantic(float64(score) / 100)
}

// efficiencyBoolSemantic is SemanticGood for true, SemanticBad for false.
func efficiencyBoolSemantic(ok bool) render.Semantic {
	if ok {
		return render.SemanticGood
	}
	return render.SemanticBad
}

// wrapToWidth word-wraps s onto lines of at most width visible runes
// each (never splitting a word unless that single word alone is
// already wider than width, in which case it is hard-split so the
// wrapper always makes progress). Used by panels whose per-row text
// can run longer than the box's own inner width (card point 6: wrap
// inside the panel instead of truncating with an ellipsis).
func wrapToWidth(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len([]rune(cur))+1+len([]rune(w)) <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	lines = append(lines, cur)
	// A single word longer than width (e.g. a long path with no
	// spaces) still needs hard-splitting, or it would overflow the
	// box just like the old truncation bug did.
	var out []string
	for _, line := range lines {
		for len([]rune(line)) > width {
			r := []rune(line)
			out = append(out, string(r[:width]))
			line = string(r[width:])
		}
		out = append(out, line)
	}
	return out
}

// renderPanelMeasured builds panel F's rows: per agent, the short
// source path, period covered, sessions read/without usage, and what
// the agent knowingly does not report (buildMeasured) -- plus the
// ignored-agents note the main view used to show as "skipped: ..."
// (card point H: moved here, dim). width is the panel's own outer box
// width (as passed to renderPanelBox): a row's line is wrapped onto as
// many continuation lines as it needs instead of being truncated with
// an ellipsis (card point 6), since an agent's full source path and
// session counts must always stay readable.
func renderPanelMeasured(d dashboardData, r render.Renderer, lang i18n.Lang, width int) []string {
	inner := width - 4
	if inner < 20 {
		inner = 20
	}
	var lines []string
	for _, row := range d.Measured {
		line := fmt.Sprintf("%s: %s (%s, %s)",
			row.Agent, row.Source, row.Period,
			i18n.T(lang, "dashboard.measured_sessions", render.FormatCountLang(row.Sessions, lang)))
		if row.SessionsNoUsage > 0 {
			line += fmt.Sprintf(", %s", i18n.T(lang, "dashboard.measured_sessions_no_usage", render.FormatCountLang(row.SessionsNoUsage, lang)))
		}
		lines = append(lines, wrapToWidth(line, inner)...)
		if row.Note != "" {
			for _, l := range wrapToWidth(row.Note, inner-2) {
				lines = append(lines, r.Muted("  "+l))
			}
		}
	}
	if len(lines) == 0 {
		lines = []string{i18n.T(lang, "dashboard.no_agents")}
	}
	if d.IgnoredNote != "" {
		lines = append(lines, r.Muted(d.IgnoredNote))
	}
	return lines
}
