package render

import (
	"strings"
	"testing"
)

func TestBarPlainHasNoAnsiEscapes(t *testing.T) {
	r := New(Options{NoColor: true, IsTTY: true})
	bar := r.Bar(0.5, 20)
	if strings.ContainsRune(bar, '\x1b') {
		t.Fatalf("Bar with NoColor=true contains an ANSI escape: %q", bar)
	}
}

func TestBarPlainOnNonTTYHasNoAnsiEscapes(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: false})
	bar := r.Bar(0.5, 20)
	if strings.ContainsRune(bar, '\x1b') {
		t.Fatalf("Bar on a non-TTY contains an ANSI escape: %q", bar)
	}
}

func TestBarColoredOnTTYContainsAnsiEscape(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true})
	bar := r.Bar(0.5, 20)
	if !strings.ContainsRune(bar, '\x1b') {
		t.Fatalf("Bar on a color TTY has no ANSI escape: %q", bar)
	}
}

func TestBarFractionIsClamped(t *testing.T) {
	r := New(Options{NoColor: true, IsTTY: true})
	if got := r.Bar(1.5, 10); strings.Count(got, "#") != 10 {
		t.Fatalf("Bar(1.5, 10) = %q, want all 10 chars filled", got)
	}
	if got := r.Bar(-0.5, 10); strings.Count(got, "#") != 0 {
		t.Fatalf("Bar(-0.5, 10) = %q, want 0 chars filled", got)
	}
}

func TestBarColoredIsSolidFillNotGradient(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true})
	// Fully filled: every character should share one accent style, not a
	// per-character gradient (one SGR color-setting sequence, not one per
	// cell).
	bar := r.Bar(1.0, 20)
	if n := countDistinctColorCodes(bar); n > 1 {
		t.Fatalf("Bar(1.0, 20) uses %d distinct color codes, want 1 (solid fill, no gradient): %q", n, bar)
	}
	// Half filled: at most 2 distinct colors (accent for filled, muted for
	// the rest), never a smooth gradient across many shades.
	half := r.Bar(0.5, 20)
	if n := countDistinctColorCodes(half); n > 2 {
		t.Fatalf("Bar(0.5, 20) uses %d distinct color codes, want <=2 (solid fill, no gradient): %q", n, half)
	}
}

// countDistinctColorCodes counts distinct ANSI SGR sequences that set a
// foreground color (CSI ... 'm' containing "38;") in s.
func countDistinctColorCodes(s string) int {
	seen := map[string]bool{}
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			break
		}
		s = s[i+2:]
		j := strings.Index(s, "m")
		if j < 0 {
			break
		}
		code := s[:j]
		s = s[j+1:]
		if strings.Contains(code, "38;") || strings.Contains(code, "38") {
			seen[code] = true
		}
	}
	return len(seen)
}

// TestBarOnColorUTF8TTYUsesBlockGlyphs verifies the real-TTY variant asked
// for by the card: on a color TTY with a UTF-8 locale, Bar renders solid
// block glyphs (filled U+2588, empty U+2591), not the ASCII '#'/'-'
// fallback. ForceASCII defaults to false (UTF-8 assumed) when unset.
func TestBarOnColorUTF8TTYUsesBlockGlyphs(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, ForceASCII: false})
	bar := r.Bar(0.5, 20)
	if strings.ContainsRune(bar, '#') || strings.ContainsRune(bar, '-') {
		t.Fatalf("Bar on a color UTF-8 TTY still uses ASCII chars: %q", bar)
	}
	if !strings.ContainsRune(bar, '\u2588') {
		t.Fatalf("Bar on a color UTF-8 TTY has no filled block glyph (U+2588): %q", bar)
	}
	if !strings.ContainsRune(bar, '\u2591') {
		t.Fatalf("Bar on a color UTF-8 TTY has no empty block glyph (U+2591): %q", bar)
	}
}

// TestBarForceASCIIKeepsASCIIEvenOnColorTTY verifies the non-UTF-8-locale
// escape hatch: a color TTY whose locale is not UTF-8 must still render
// plain '#'/'-' (the card's third ASCII condition, alongside NO_COLOR and
// non-TTY).
func TestBarForceASCIIKeepsASCIIEvenOnColorTTY(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, ForceASCII: true})
	bar := r.Bar(0.5, 20)
	if strings.ContainsRune(bar, '\u2588') || strings.ContainsRune(bar, '\u2591') {
		t.Fatalf("Bar with ForceASCII=true still uses block glyphs: %q", bar)
	}
	if !strings.ContainsRune(bar, '#') {
		t.Fatalf("Bar with ForceASCII=true has no ASCII filled char: %q", bar)
	}
}

func TestTablePlainHasNoAnsiEscapesButKeepsBorders(t *testing.T) {
	r := New(Options{NoColor: true, IsTTY: true})
	out := r.Table([]string{"session", "create"}, [][]string{{"a.jsonl", "50000"}})
	if strings.ContainsRune(out, '\x1b') {
		t.Fatalf("Table with NoColor=true contains an ANSI escape: %q", out)
	}
	if !strings.Contains(out, "session") || !strings.Contains(out, "a.jsonl") {
		t.Fatalf("Table output missing expected cells: %q", out)
	}
}
