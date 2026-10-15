package render

import "testing"

func TestVerticalBarsHeightMatchesRelativeMagnitude(t *testing.T) {
	lines := VerticalBars([]float64{0, 5, 10}, 4)
	if len(lines) != 4 {
		t.Fatalf("len(lines) = %d, want 4", len(lines))
	}
	for _, l := range lines {
		if len([]rune(l)) != 3 {
			t.Fatalf("line %q has %d columns, want 3", l, len([]rune(l)))
		}
	}
	// column 0 (value 0) must be blank on every row.
	for i, l := range lines {
		if r := []rune(l)[0]; r != ' ' {
			t.Fatalf("row %d col 0 (value 0) = %q, want blank", i, string(r))
		}
	}
	// column 2 (the max value) must reach the top row (row 0).
	if r := []rune(lines[0])[2]; r == ' ' {
		t.Fatal("row 0 (top) col 2 (max value) is blank, want it filled")
	}
	// column 1 (half the max) must NOT reach the top row.
	if r := []rune(lines[0])[1]; r != ' ' {
		t.Fatalf("row 0 (top) col 1 (half of max) = %q, want blank (shorter bar)", string(r))
	}
}

func TestVerticalBarsEmptyInputIsNil(t *testing.T) {
	if got := VerticalBars(nil, 4); got != nil {
		t.Fatalf("VerticalBars(nil, 4) = %v, want nil", got)
	}
	if got := VerticalBars([]float64{1, 2}, 0); got != nil {
		t.Fatalf("VerticalBars(values, 0) = %v, want nil", got)
	}
}
