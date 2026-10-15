package render

// VerticalBars renders values as a multi-row vertical bar chart: rows
// lines, each exactly len(values) columns wide (one column per value),
// built bottom-up using the same 8 sub-cell block glyphs Sparkline
// uses (sparkGlyphs: 1/8 through a full block), so a bar's height is
// not limited to 8 discrete buckets total -- it is representable
// across however many rows the caller gives it (card: "the histogram
// grows to fill remaining rows"). Returned top row first (index 0),
// matching normal top-to-bottom terminal line order. nil for no
// values or rows <= 0.
func VerticalBars(values []float64, rows int) []string {
	if len(values) == 0 || rows <= 0 {
		return nil
	}
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	grid := make([][]rune, rows)
	for i := range grid {
		grid[i] = make([]rune, len(values))
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}
	for col, v := range values {
		if max <= 0 || v <= 0 {
			continue
		}
		frac := v / max
		eighths := int(frac*float64(rows)*8 + 0.5)
		if eighths > rows*8 {
			eighths = rows * 8
		}
		fullRows := eighths / 8
		remainder := eighths % 8
		for r := 0; r < rows; r++ {
			fromBottom := rows - 1 - r
			switch {
			case fromBottom < fullRows:
				grid[r][col] = sparkGlyphs[len(sparkGlyphs)-1]
			case fromBottom == fullRows && remainder > 0:
				grid[r][col] = sparkGlyphs[remainder-1]
			}
		}
	}
	out := make([]string, rows)
	for i, l := range grid {
		out[i] = string(l)
	}
	return out
}
