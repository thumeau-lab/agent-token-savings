package render

import "testing"

func TestFormatCountBelowThousandIsPlainInteger(t *testing.T) {
	if got := FormatCount(412); got != "412" {
		t.Fatalf("FormatCount(412) = %q, want %q", got, "412")
	}
	if got := FormatCount(0); got != "0" {
		t.Fatalf("FormatCount(0) = %q, want %q", got, "0")
	}
}

func TestFormatCountMillionsUsesThreeSignificantDigits(t *testing.T) {
	if got := FormatCount(9_870_000); got != "9.87 M" {
		t.Fatalf("FormatCount(9_870_000) = %q, want %q", got, "9.87 M")
	}
	if got := FormatCount(64_700_000); got != "64.7 M" {
		t.Fatalf("FormatCount(64_700_000) = %q, want %q", got, "64.7 M")
	}
}

func TestFormatCountThousandsAndBillions(t *testing.T) {
	if got := FormatCount(1_500); got != "1.50 K" {
		t.Fatalf("FormatCount(1_500) = %q, want %q", got, "1.50 K")
	}
	if got := FormatCount(1_000_000_000); got != "1.00 B" {
		t.Fatalf("FormatCount(1_000_000_000) = %q, want %q", got, "1.00 B")
	}
}

func TestFormatCountNegativeKeepsSign(t *testing.T) {
	if got := FormatCount(-9_870_000); got != "-9.87 M" {
		t.Fatalf("FormatCount(-9_870_000) = %q, want %q", got, "-9.87 M")
	}
}

func TestFormatUSDTwoDecimalsWithDollarSign(t *testing.T) {
	if got := FormatUSD(12.40); got != "$12.40" {
		t.Fatalf("FormatUSD(12.40) = %q, want %q", got, "$12.40")
	}
	if got := FormatUSD(0); got != "$0.00" {
		t.Fatalf("FormatUSD(0) = %q, want %q", got, "$0.00")
	}
}

func TestFormatUSDGroupsThousands(t *testing.T) {
	if got := FormatUSD(1234.5); got != "$1,234.50" {
		t.Fatalf("FormatUSD(1234.5) = %q, want %q", got, "$1,234.50")
	}
}

// TestFormatBytesGluesScaleLetterToUnit verifies the byte-size formatter
// glues the K/M/B scale letter directly onto the "B" unit (e.g. "26.2 KB",
// never "26.2 K B" wrapping onto its own line), and keeps a space before
// "B" when there is no scale letter to glue onto.
func TestFormatBytesGluesScaleLetterToUnit(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{933, "933 B"},
		{0, "0 B"},
		{26_200, "26.2 KB"},
		{9_870_000, "9.87 MB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.n); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

