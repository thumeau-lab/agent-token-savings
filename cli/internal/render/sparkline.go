package render

// sparkGlyphs are the 8 block levels used by Sparkline, lowest to
// highest (U+2581 to U+2588): a plain-ASCII-free but still simple block
// sparkline (the card allows braille or block; block was chosen for
// simplicity -- each glyph is one cell, no 2x4 dot-matrix encoding).
var sparkGlyphs = []rune{'\u2581', '\u2582', '\u2583', '\u2584', '\u2585', '\u2586', '\u2587', '\u2588'}

// Sparkline renders values as a one-line block-glyph sparkline exactly
// width glyphs wide: values are bucketed (averaged) down to width
// buckets when there are more values than width, or each value maps to
// its own glyph when there are fewer. Each bucket's average is
// normalized against the series' own min/max to pick one of 8 levels; a
// perfectly flat series (max == min) renders at the lowest level for
// every glyph instead of dividing by zero.
func Sparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return ""
	}

	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	buckets := bucketAverage(values, width)
	out := make([]rune, len(buckets))
	for i, v := range buckets {
		out[i] = levelFor(v, min, max)
	}
	return string(out)
}

// bucketAverage splits values into exactly n contiguous buckets (as even
// as integer division allows) and averages each one, so a series longer
// than the requested width downsamples instead of overflowing it. A
// series shorter than n is returned unchanged (one bucket per value):
// Sparkline then renders fewer glyphs than width, which callers that
// need an exact width should pad themselves (card panels pad with
// trailing space, not a flat glyph, so a short history is visually
// distinguishable from a flat one).
func bucketAverage(values []float64, n int) []float64 {
	if len(values) <= n {
		return values
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		lo := i * len(values) / n
		hi := (i + 1) * len(values) / n
		if hi <= lo {
			hi = lo + 1
		}
		var sum float64
		count := 0
		for j := lo; j < hi && j < len(values); j++ {
			sum += values[j]
			count++
		}
		if count > 0 {
			out[i] = sum / float64(count)
		}
	}
	return out
}

// levelFor maps v into one of sparkGlyphs' 8 levels given the series'
// min/max, flooring at the lowest glyph when max == min (a flat series,
// which would otherwise divide by zero).
func levelFor(v, min, max float64) rune {
	if max <= min {
		return sparkGlyphs[0]
	}
	frac := (v - min) / (max - min)
	idx := int(frac * float64(len(sparkGlyphs)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sparkGlyphs) {
		idx = len(sparkGlyphs) - 1
	}
	return sparkGlyphs[idx]
}
