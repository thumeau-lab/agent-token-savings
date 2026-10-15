// Package render turns ats data into terminal output: a bordered Lip Gloss
// card per agent (bordered box, title, one-line verdict, 2-column stat
// grid, solid accent-color bars, optional small table) when stdout is a
// TTY and NO_COLOR is unset; plain ASCII (no ANSI escapes at all)
// otherwise. See docs' "Look and feel" section.
package render

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Options controls whether a Renderer may emit color/styling.
type Options struct {
	// NoColor mirrors the NO_COLOR convention (https://no-color.org/): when
	// true, never emit ANSI escapes regardless of IsTTY.
	NoColor bool
	// IsTTY is whether stdout is a terminal. When false, ats always renders
	// plain text (no ANSI), independently of NoColor.
	IsTTY bool
	// Dark reports whether the terminal has a dark background, as reported
	// by lipgloss.HasDarkBackground(os.Stdin, os.Stdout) in main.go. It
	// selects which half of each adaptive accent/muted color pair to use,
	// so output reads on both light and dark terminals. Ignored when the
	// renderer is not Colored().
	Dark bool
	// ForceASCII is true when the terminal's locale is not UTF-8 (see
	// LocaleIsUTF8): it forces Bar back to the plain '#'/'-' glyphs even
	// on a color TTY, since the solid block glyphs would not render
	// correctly outside a UTF-8 locale. The card's rule is ASCII only on
	// NO_COLOR, a non-TTY, or a non-UTF-8 locale -- the first two are
	// already covered by Colored() being false; this field covers the third.
	ForceASCII bool
}

// Renderer renders the pieces of ats' inline output (bars, tables, cards)
// honoring Options.
type Renderer struct {
	opts Options
}

// New builds a Renderer for the given Options.
func New(opts Options) Renderer {
	return Renderer{opts: opts}
}

// Colored reports whether this renderer may emit ANSI escapes.
func (r Renderer) Colored() bool {
	return r.opts.IsTTY && !r.opts.NoColor
}

// colors returns the accent (titles, selection -- one warm color, used
// sparingly per the Claude-Code-like restraint rule), label (primary
// text: stat/bar labels, big numbers -- never below #a8a8a8 on a dark
// background, per the contrast rule) and dim (secondary hints only:
// subtitles, borders, skip notes) colors for this renderer, picking the
// light or dark half of each adaptive pair via opts.Dark.
func (r Renderer) colors() (accent, label, dim color.Color) {
	ld := lipgloss.LightDark(r.opts.Dark)
	accent = ld(lipgloss.Color("#b35f3f"), lipgloss.Color("#d97757")) // warm orange, the one accent
	label = ld(lipgloss.Color("#3a3a3a"), lipgloss.Color("#c8c8c8"))  // primary text, readable on both
	dim = ld(lipgloss.Color("#767676"), lipgloss.Color("#9E9E9E"))   // secondary hints only
	return accent, label, dim
}

// ChromeColors exports colors() for callers outside the package that
// style their own chrome consistently with the card's palette (the TUI's
// header/footer, which are not a Card themselves but must use the same
// accent/label/dim colors a Card does).
func (r Renderer) ChromeColors() (accent, label, dim color.Color) {
	return r.colors()
}

// barGlyphFilled and barGlyphEmpty are the solid block glyphs used on a
// color TTY with a UTF-8 locale: a full block for the filled portion, a
// light shade (dim) block for the empty portion, matching the card's
// "filled, empty in muted grey" look.
const (
	barGlyphFilled = "\u2588" // █ FULL BLOCK
	barGlyphEmpty  = "\u2591" // ░ LIGHT SHADE
)

// Bar renders a horizontal bar of `width` characters, `frac` (clamped to
// [0,1]) of it filled. ASCII ('#' filled, '-' empty) when not Colored, or
// when Colored but ForceASCII (non-UTF-8 locale). Otherwise (a real color
// TTY with a UTF-8 locale) solid block glyphs: filled in fill (the
// renderer's accent when none is given -- pass HitRateSemantic/
// CostBucketColor's result for a meter that should carry meaning instead
// of the generic accent), empty in dim grey -- no gradient either way.
func (r Renderer) Bar(frac float64, width int, fill ...color.Color) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	if width <= 0 {
		width = 1
	}
	filled := int(frac*float64(width) + 0.5)
	if filled > width {
		filled = width
	}

	if !r.Colored() {
		return strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	}

	filledGlyph, emptyGlyph := "#", "-"
	if !r.opts.ForceASCII {
		filledGlyph, emptyGlyph = barGlyphFilled, barGlyphEmpty
	}

	accent, _, dim := r.colors()
	fillColor := accent
	if len(fill) > 0 {
		fillColor = fill[0]
	}
	var out string
	if filled > 0 {
		out += lipgloss.NewStyle().Foreground(fillColor).Render(strings.Repeat(filledGlyph, filled))
	}
	if empty := width - filled; empty > 0 {
		out += lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat(emptyGlyph, empty))
	}
	return out
}

// Muted renders s in the card's dim grey when Colored(), unchanged
// otherwise (plain text, NO_COLOR, or non-TTY): used for secondary text
// that should read as de-emphasized next to a card, e.g. a skipped
// agent's one-line note. Never used for a primary label or big number
// (see the contrast rule on the `dim` color in colors()).
func (r Renderer) Muted(s string) string {
	if !r.Colored() {
		return s
	}
	_, _, dim := r.colors()
	return lipgloss.NewStyle().Foreground(dim).Render(s)
}

// Table renders a simple bordered table. Border box-drawing characters are
// always kept (allowed even in plain output); only color is gated by Colored().
func (r Renderer) Table(headers []string, rows [][]string) string {
	t := table.New().Headers(headers...).Rows(rows...)
	if r.Colored() {
		accent, _, _ := r.colors()
		headerStyle := lipgloss.NewStyle().Foreground(accent)
		t = t.StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return lipgloss.NewStyle()
		})
	}
	return t.String()
}
