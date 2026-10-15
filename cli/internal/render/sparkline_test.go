package render

import "testing"

func TestSparklineEmptyValuesReturnsEmptyString(t *testing.T) {
	if got := Sparkline(nil, 10); got != "" {
		t.Fatalf("Sparkline(nil, 10) = %q, want empty", got)
	}
}

func TestSparklineOneBarPerValueWhenWidthMatches(t *testing.T) {
	got := Sparkline([]float64{0, 1}, 2)
	if len([]rune(got)) != 2 {
		t.Fatalf("Sparkline([0,1], 2) = %q, want 2 glyphs", got)
	}
}

func TestSparklineFlatSeriesUsesLowestLevel(t *testing.T) {
	got := Sparkline([]float64{5, 5, 5}, 3)
	runes := []rune(got)
	want := sparkGlyphs[0]
	for _, r := range runes {
		if r != want {
			t.Fatalf("Sparkline(flat) = %q, want every glyph to be %q (lowest level, no division by zero)", got, string(want))
		}
	}
}

func TestSparklineHighestValueUsesTopGlyph(t *testing.T) {
	got := []rune(Sparkline([]float64{0, 10}, 2))
	top := sparkGlyphs[len(sparkGlyphs)-1]
	if got[1] != top {
		t.Fatalf("Sparkline([0,10],2)[1] = %q, want top glyph %q", string(got[1]), string(top))
	}
	if got[0] != sparkGlyphs[0] {
		t.Fatalf("Sparkline([0,10],2)[0] = %q, want bottom glyph %q", string(got[0]), string(sparkGlyphs[0]))
	}
}

func TestSparklineDownsamplesToRequestedWidth(t *testing.T) {
	values := make([]float64, 30)
	for i := range values {
		values[i] = float64(i)
	}
	got := []rune(Sparkline(values, 10))
	if len(got) != 10 {
		t.Fatalf("len(Sparkline(30 values, 10)) = %d, want 10", len(got))
	}
}
