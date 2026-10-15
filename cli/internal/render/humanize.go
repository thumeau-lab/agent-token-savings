package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FormatCount renders a token/call count humanized to 3 significant digits
// with a K/M/B suffix (e.g. 9_870_000 -> "9.87 M", 64_700_000 -> "64.7 M").
// Below 1000 it is printed as a plain integer. Used by card stat grids so
// large numbers stay readable instead of raw digit runs.
func FormatCount(n int64) string {
	neg := n < 0
	abs := n
	if neg {
		abs = -abs
	}
	var scaled float64
	var suffix string
	switch {
	case abs < 1_000:
		s := strconv.FormatInt(n, 10)
		return s
	case abs < 1_000_000:
		scaled, suffix = float64(abs)/1_000, "K"
	case abs < 1_000_000_000:
		scaled, suffix = float64(abs)/1_000_000, "M"
	default:
		scaled, suffix = float64(abs)/1_000_000_000, "B"
	}

	decimals := 0
	switch {
	case scaled < 10:
		decimals = 2
	case scaled < 100:
		decimals = 1
	}
	out := strconv.FormatFloat(scaled, 'f', decimals, 64)
	if neg {
		out = "-" + out
	}
	return out + " " + suffix
}

// FormatUSD renders a dollar amount with exactly two decimals, a leading
// "$" and comma-grouped thousands (e.g. 1234.5 -> "$1,234.50"). Negative
// amounts keep the sign before the "$".
func FormatUSD(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	// Round at 2 decimals before splitting, so 0.005 rounding edge cases
	// land on the same cent fmt.Sprintf would pick.
	v = math.Round(v*100) / 100
	whole := int64(v)
	cents := int64(math.Round((v - float64(whole)) * 100))
	if cents == 100 { // rounding carried into the next whole dollar
		whole++
		cents = 0
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s$%s.%02d", sign, groupThousands(whole), cents)
}

// groupThousands inserts "," every 3 digits from the right, e.g. 1234 -> "1,234".
func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

// FormatBytes renders a byte count the same way FormatCount does, with the
// "B" unit glued directly onto the K/M/B scale letter (e.g. "26.2 KB", not
// "26.2 K B" -- the latter has an extra word-break Lip Gloss can wrap on,
// pushing "B" onto its own line in a narrow column). Below 1000, where
// FormatCount has no scale letter to glue onto, " B" is appended with a
// space (e.g. "933 B").
func FormatBytes(n int64) string {
	s := FormatCount(n)
	if strings.Contains(s, " ") {
		return s + "B"
	}
	return s + " B"
}
