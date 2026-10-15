package render

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// Stat is one label/value pair in a card's 2-column grid (e.g. "Sessions"
// / "12"). Values are expected pre-formatted (humanized numbers, aligned
// percentages) by the caller via FormatCount/FormatUSD.
type Stat struct {
	Label string
	Value string
}

// BarStat is one accent-colored bar row inside a card: a label, a
// fraction in [0,1], and the trailing annotation text shown after the bar
// (e.g. a percentage or a humanized value). Color, when non-nil,
// overrides the renderer's single accent for this one bar -- used for the
// hit-rate meter (HitRateSemantic/SemanticColor) and the per-bucket cost
// shares (CostBucketColor), so a reader can tell bars apart by color
// instead of every bar sharing one cyan.
type BarStat struct {
	Label      string
	Frac       float64
	Annotation string
	Color      color.Color
}

// CardTable is an optional small table shown inside a card, e.g. "worst
// cache breaks" (session, time, tokens rewritten).
type CardTable struct {
	Title   string
	Headers []string
	Rows    [][]string
}

// LegendItem is one entry in a card's color legend: a label and the
// color its matching bar/swatch uses, so a reader can tell which bucket
// is which without memorizing an arbitrary color order.
type LegendItem struct {
	Label string
	Color color.Color
}

// Card is one agent's full styled block: a rounded box whose top border
// embeds the agent name as title (btop style: "╭─ 1 Cache ────╮", Key
// optional), a one-line verdict in plain words, a 2-column stat grid,
// solid color-coded bars with an optional legend, and an optional table.
type Card struct {
	// Key is the btop-style shortcut digit shown right next to Title in
	// the top border (e.g. "1"); empty when the card has no tab shortcut
	// of its own (e.g. a standalone `ats cache` card).
	Key      string
	Title    string
	Subtitle string
	Verdict  string
	Stats    []Stat
	Bars     []BarStat
	// Graph is an optional pre-rendered block (e.g. a per-day history
	// sparkline) shown after Bars/Legend and before Table. Pre-rendered
	// by the caller (cliapp builds it from render.Sparkline) rather than
	// a dedicated field set, since its exact layout (axis labels, two
	// stacked series) is specific to the Cache panel.
	Graph  string
	Table  *CardTable
	Legend []LegendItem
}

// The stat grid and bar rows use a label column sized to the longest
// label actually present in the card (floored at these minimums), so a
// long component name (e.g. "MCP servers (config.yaml)") is never
// truncated or wrapped onto a second line -- see labelColumnWidth. Bars
// grow with the available width (see renderBarRow) since a wider bar is
// more useful on a wide terminal.
const (
	cardLabelWidthMin = 14
	cardValueWidth    = 9
	cardGridGutter    = 3

	cardBarLabelWidthMin = 12
	cardBarAnnotWidth    = 7
)

// RenderCard renders one Card as a rounded box with the title embedded in
// the top border, sized to width. Intended for Colored() renderers only:
// callers keep the existing plain key/value text path for non-TTY,
// NO_COLOR and --json output.
func (r Renderer) RenderCard(c Card, width int) string {
	if width < 40 {
		width = 40
	}
	accent, label, dim := r.colors()
	innerWidth := width - 4 // 2 border chars + 1 padding column each side
	labelStyle := lipgloss.NewStyle().Foreground(label)
	dimStyle := lipgloss.NewStyle().Foreground(dim)

	var blocks []string
	if c.Subtitle != "" {
		blocks = append(blocks, dimStyle.Render(c.Subtitle))
	}
	if c.Verdict != "" {
		blocks = append(blocks, lipgloss.NewStyle().Bold(true).Width(innerWidth).Render(c.Verdict))
	}
	if len(c.Stats) > 0 {
		blocks = append(blocks, renderStatGrid(c.Stats, labelStyle, innerWidth))
	}
	if len(c.Bars) > 0 {
		labelWidth := labelColumnWidth(c.Bars, cardBarLabelWidthMin)
		var barLines []string
		for _, b := range c.Bars {
			barLines = append(barLines, r.renderBarRow(b, innerWidth, labelWidth, labelStyle))
		}
		blocks = append(blocks, strings.Join(barLines, "\n"))
	}
	if len(c.Legend) > 0 {
		blocks = append(blocks, r.renderLegend(c.Legend))
	}
	if c.Graph != "" {
		blocks = append(blocks, c.Graph)
	}
	if c.Table != nil && len(c.Table.Rows) > 0 {
		var tbl []string
		if c.Table.Title != "" {
			tbl = append(tbl, dimStyle.Render(c.Table.Title))
		}
		tbl = append(tbl, r.Table(c.Table.Headers, c.Table.Rows))
		blocks = append(blocks, strings.Join(tbl, "\n"))
	}

	body := strings.Join(blocks, "\n\n")
	return r.renderBoxWithTitle(c.Key, c.Title, body, width, accent, dim, innerWidth)
}

// renderBoxWithTitle manually draws a rounded box whose top border embeds
// "<key> <title>" (btop style), since Lip Gloss v2 has no built-in
// title-in-border support. The bottom and side borders are drawn the same
// way lipgloss.RoundedBorder()'s own runes look, so the box reads as one
// consistent rounded frame.
func (r Renderer) renderBoxWithTitle(key, title, body string, width int, accent, dim color.Color, contentWidth int) string {
	b := lipgloss.RoundedBorder()
	dimStyle := lipgloss.NewStyle().Foreground(dim)

	titleText := title
	if key != "" {
		titleText = key + " " + title
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accent)
	styledTitle := titleStyle.Render(titleText)
	titleVisibleLen := lipgloss.Width(titleText)

	// corner + dash + space + title + space + dashes + corner == width
	dashesAfter := width - 5 - titleVisibleLen
	if dashesAfter < 1 {
		dashesAfter = 1
	}
	var sb strings.Builder
	sb.WriteString(dimStyle.Render(b.TopLeft + b.Top + " "))
	sb.WriteString(styledTitle)
	sb.WriteString(dimStyle.Render(" " + strings.Repeat(b.Top, dashesAfter) + b.TopRight))
	sb.WriteString("\n")

	for _, line := range strings.Split(body, "\n") {
		padded := lipgloss.NewStyle().Width(contentWidth).Render(line)
		sb.WriteString(dimStyle.Render(b.Left))
		sb.WriteString(" ")
		sb.WriteString(padded)
		sb.WriteString(" ")
		sb.WriteString(dimStyle.Render(b.Right))
		sb.WriteString("\n")
	}

	sb.WriteString(dimStyle.Render(b.BottomLeft + strings.Repeat(b.Bottom, width-2) + b.BottomRight))
	return sb.String()
}

// renderLegend renders one line naming each bucket next to a swatch in
// its matching color (e.g. "■ write  ■ read  ■ input  ■ output"), so the
// cost-share bars' colors are explained once instead of left for the
// reader to guess.
func (r Renderer) renderLegend(items []LegendItem) string {
	var parts []string
	for _, it := range items {
		swatch := lipgloss.NewStyle().Foreground(it.Color).Render("\u25a0")
		parts = append(parts, swatch+" "+it.Label)
	}
	return strings.Join(parts, "  ")
}

// labelColumnWidth returns the widest BarStat.Label rune count in bars,
// floored at min: the column never shrinks below the card's usual
// minimum, but grows to fit a longer label instead of truncating or
// wrapping it.
func labelColumnWidth(bars []BarStat, min int) int {
	width := min
	for _, b := range bars {
		if l := len([]rune(b.Label)); l > width {
			width = l
		}
	}
	return width
}

// renderStatGrid lays Stats out in a 2-column grid, labels in the
// renderer's primary label color and values right-aligned within their
// column so percentages and humanized numbers line up down the card.
// Both columns are sized to the longest label/value in stats (floored at
// cardLabelWidthMin/cardValueWidth) so a long label (e.g.
// "API-equivalent cost") or a long value (e.g. "6.56 K tok") is never
// wrapped onto a second line. When the resulting 2-column row would not
// fit within width (e.g. a long French label like "Ecritures ecart
// moyen (5-60 min)" at a narrow panel width), the grid falls back to one
// column per row instead of letting lipgloss's fixed-width box wrap the
// row onto a second line with no border around the overflow (the exact
// bug the 170x46 wide-layout TTL panel hit). width <= 0 (unknown) keeps
// the 2-column layout, matching the caller's own width<40 floor.
func renderStatGrid(stats []Stat, labelStyle lipgloss.Style, width int) string {
	labelWidth, valueWidth := cardLabelWidthMin, cardValueWidth
	for _, s := range stats {
		if l := len([]rune(s.Label)); l > labelWidth {
			labelWidth = l
		}
		if l := len([]rune(s.Value)); l > valueWidth {
			valueWidth = l
		}
	}
	twoColWidth := 2*(labelWidth+valueWidth+1) + cardGridGutter
	if width > 0 && twoColWidth > width {
		var rows []string
		for _, s := range stats {
			rows = append(rows, statCell(s, labelWidth, valueWidth, labelStyle))
		}
		return strings.Join(rows, "\n")
	}
	var rows []string
	for i := 0; i < len(stats); i += 2 {
		row := statCell(stats[i], labelWidth, valueWidth, labelStyle)
		if i+1 < len(stats) {
			row += strings.Repeat(" ", cardGridGutter)
			row += statCell(stats[i+1], labelWidth, valueWidth, labelStyle)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

// statCell renders one label/value pair: the label padded to labelWidth
// then one space then the right-aligned value. The explicit space
// matters even though labelStyle.Width(labelWidth) usually already pads
// the label: when the label's rune length exactly equals labelWidth
// (the longest label in the card, e.g. a long French translation),
// Width() adds no padding at all, and without this separator the label
// would glue directly onto the value with no space between them.
func statCell(s Stat, labelWidth, valueWidth int, labelStyle lipgloss.Style) string {
	label := labelStyle.Width(labelWidth).Render(s.Label)
	value := lipgloss.NewStyle().Bold(true).Width(valueWidth).Align(lipgloss.Right).Render(s.Value)
	return label + " " + value
}

// renderBarRow lays out one label, a solid-fill bar and a trailing
// right-aligned annotation within width, using labelWidth for the label
// column (see labelColumnWidth: sized to the card's longest bar label,
// so it is never truncated). The bar grows to fill whatever width
// remains, so it reads better on wider terminals. b.Color overrides the
// renderer's accent for this one bar when set (semantic/cost-bucket
// meters); nil falls back to Bar's own default (the accent).
func (r Renderer) renderBarRow(b BarStat, width, labelWidth int, labelStyle lipgloss.Style) string {
	barWidth := width - labelWidth - cardBarAnnotWidth - 2
	if barWidth < 4 {
		barWidth = 4
	}
	label := labelStyle.Width(labelWidth).MaxWidth(labelWidth).Render(b.Label)
	var bar string
	if b.Color != nil {
		bar = r.Bar(b.Frac, barWidth, b.Color)
	} else {
		bar = r.Bar(b.Frac, barWidth)
	}
	ann := lipgloss.NewStyle().Width(cardBarAnnotWidth).Align(lipgloss.Right).Render(b.Annotation)
	return label + " " + bar + " " + ann
}
