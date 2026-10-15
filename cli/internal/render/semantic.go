package render

import (
	"fmt"
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

// Semantic is a traffic-light bucket for a metric that reads better as
// good/warn/bad than as a single neutral color (currently: cache hit
// rate). Named constants instead of inline thresholds so the policy is
// visible and unit-testable on its own, independent of actual rendering.
type Semantic int

const (
	SemanticGood Semantic = iota
	SemanticWarn
	SemanticBad
)

// HitRateGoodThreshold and HitRateWarnThreshold are the cache hit-rate
// cutoffs for the Cache panel's meter: at or above Good is green, at or
// above Warn (but below Good) is yellow, below Warn is red. Chosen from
// docs/measurements.md's own numbers (healthy installs sit at ~90%+ hit
// rate; anything under 60% means most of the prefix is being rewritten).
const (
	HitRateGoodThreshold = 0.85
	HitRateWarnThreshold = 0.60
)

// HitRateSemantic buckets a hit rate fraction (0-1) into Good/Warn/Bad
// per the thresholds above. Both boundaries are inclusive on their
// upper bucket (>= Good is Good, >= Warn is Warn), so a value never
// falls through a gap between HitRateSemantic and the thresholds it
// quotes in documentation or tests.
func HitRateSemantic(frac float64) Semantic {
	switch {
	case frac >= HitRateGoodThreshold:
		return SemanticGood
	case frac >= HitRateWarnThreshold:
		return SemanticWarn
	default:
		return SemanticBad
	}
}

// SemanticColor returns the adaptive (light/dark aware) color for one
// Semantic bucket: green (Good), yellow (Warn), red (Bad).
func (r Renderer) SemanticColor(s Semantic) color.Color {
	ld := lipgloss.LightDark(r.opts.Dark)
	switch s {
	case SemanticGood:
		return ld(lipgloss.Color("#1a7a1a"), lipgloss.Color("#5fd75f"))
	case SemanticWarn:
		return ld(lipgloss.Color("#9a7a00"), lipgloss.Color("#e5c158"))
	default:
		return ld(lipgloss.Color("#b33a3a"), lipgloss.Color("#ff6b6b"))
	}
}

// costBucketOrder fixes the (write, read, input, output) order every
// cost-share bar/legend uses, so the bucket-to-color mapping never
// silently shifts between a card's bars and its legend.
var costBucketOrder = []string{"cache_write", "cache_read", "input", "output"}

// CostBucketColor returns one distinct, fixed color per API-equivalent
// cost-share bucket (write/read/input/output), replacing the single
// accent color every bar used to share: readers can now tell which
// bucket a bar/legend swatch belongs to by color alone. Colors are
// fixed (not light/dark adaptive pairs): all four are mid-brightness
// hues chosen to stay legible on both a light and a dark background.
func (r Renderer) CostBucketColor(bucket string) color.Color {
	switch bucket {
	case "cache_write":
		return lipgloss.Color("#d97757") // the one warm accent, reused: write is the usual cost driver
	case "cache_read":
		return lipgloss.Color("#4da6a6")
	case "input":
		return lipgloss.Color("#5b8dd6")
	case "output":
		return lipgloss.Color("#9d7bd8")
	default:
		_, _, dim := r.colors()
		return dim
	}
}

// colorKey renders c against a fixed background-less profile into a
// stable comparable string, for tests that only need to assert two
// colors differ (not what their exact escape sequence is).
func colorKey(c color.Color) string {
	cr, cg, cb, ca := c.RGBA()
	return fmt.Sprintf("%d-%d-%d-%d", cr, cg, cb, ca)
}
