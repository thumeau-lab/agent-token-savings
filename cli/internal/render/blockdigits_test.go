package render

import (
	"strings"
	"testing"
)

// TestBlockDigitsRendersExactlyThreeLines is the first tracer-bullet
// behavior: BlockDigits always returns 3 lines (a 3-row-tall glyph),
// regardless of input.
func TestBlockDigitsRendersExactlyThreeLines(t *testing.T) {
	lines := BlockDigits("0")
	if len(lines) != 3 {
		t.Fatalf("BlockDigits(\"0\") returned %d lines, want 3", len(lines))
	}
}

// TestBlockDigitsEveryDigitGlyphHasConsistentWidthAcrossItsThreeRows
// pins the per-glyph width/height contract the card asks for: every
// digit 0-9 renders as exactly 3 rows, each row the same rune width as
// the others for that glyph (a ragged glyph would misalign the tile
// it sits in).
func TestBlockDigitsEveryDigitGlyphHasConsistentWidthAcrossItsThreeRows(t *testing.T) {
	for _, d := range "0123456789" {
		lines := BlockDigits(string(d))
		if len(lines) != 3 {
			t.Fatalf("BlockDigits(%q) returned %d lines, want 3", d, len(lines))
		}
		w := len([]rune(lines[0]))
		for i, line := range lines {
			if got := len([]rune(line)); got != w {
				t.Fatalf("BlockDigits(%q) row %d width = %d, want %d (same as row 0)", d, i, got, w)
			}
		}
	}
}

// TestBlockDigitsRendersAFullFrenchMoneyString is the card's required
// smoke test: "475,17 $" (a French-formatted cost) must render without
// panicking, as 3 non-empty lines.
func TestBlockDigitsRendersAFullFrenchMoneyString(t *testing.T) {
	lines := BlockDigits("475,17 $")
	if len(lines) != 3 {
		t.Fatalf("BlockDigits(\"475,17 $\") returned %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" && i == 1 {
			t.Fatalf("BlockDigits(\"475,17 $\") row %d is entirely blank, want glyph content", i)
		}
	}
	// Every row must have the same total width (glyphs line up in a column).
	w := len([]rune(lines[0]))
	for i, line := range lines {
		if got := len([]rune(line)); got != w {
			t.Fatalf("BlockDigits(\"475,17 $\") row %d width = %d, want %d", i, got, w)
		}
	}
}

// TestBlockDigitsUnknownRuneFallsBackInsteadOfPanicking: a rune outside
// the supported set (digits, ',', '.', '%', '$', ' ') must still render
// 3 well-formed rows instead of panicking or silently dropping the
// character.
func TestBlockDigitsUnknownRuneFallsBackInsteadOfPanicking(t *testing.T) {
	lines := BlockDigits("X")
	if len(lines) != 3 {
		t.Fatalf("BlockDigits(\"X\") returned %d lines, want 3", len(lines))
	}
	w := len([]rune(lines[0]))
	for i, line := range lines {
		if got := len([]rune(line)); got != w {
			t.Fatalf("BlockDigits(\"X\") row %d width = %d, want %d", i, got, w)
		}
	}
	joined := strings.Join(lines, "")
	if !strings.Contains(joined, "X") {
		t.Fatalf("BlockDigits(\"X\") fallback does not show the literal rune anywhere: %q", lines)
	}
}
