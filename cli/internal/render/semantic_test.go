package render

import "testing"

// Color thresholds are unit-tested at the public boundary (HitRateSemantic),
// independent of actual ANSI rendering: the golden tests already pin the
// exact rendered colors, this test pins the policy of which bucket a given
// hit rate falls into.

func TestHitRateSemanticGoodAtAndAboveThreshold(t *testing.T) {
	if got := HitRateSemantic(HitRateGoodThreshold); got != SemanticGood {
		t.Fatalf("HitRateSemantic(%v) = %v, want SemanticGood", HitRateGoodThreshold, got)
	}
	if got := HitRateSemantic(1.0); got != SemanticGood {
		t.Fatalf("HitRateSemantic(1.0) = %v, want SemanticGood", got)
	}
}

func TestHitRateSemanticWarnBetweenThresholds(t *testing.T) {
	mid := (HitRateGoodThreshold + HitRateWarnThreshold) / 2
	if got := HitRateSemantic(mid); got != SemanticWarn {
		t.Fatalf("HitRateSemantic(%v) = %v, want SemanticWarn", mid, got)
	}
	if got := HitRateSemantic(HitRateWarnThreshold); got != SemanticWarn {
		t.Fatalf("HitRateSemantic(%v) = %v, want SemanticWarn (boundary is inclusive)", HitRateWarnThreshold, got)
	}
}

func TestHitRateSemanticBadBelowWarnThreshold(t *testing.T) {
	below := HitRateWarnThreshold - 0.01
	if got := HitRateSemantic(below); got != SemanticBad {
		t.Fatalf("HitRateSemantic(%v) = %v, want SemanticBad", below, got)
	}
	if got := HitRateSemantic(0); got != SemanticBad {
		t.Fatalf("HitRateSemantic(0) = %v, want SemanticBad", got)
	}
}

// TestCostBucketColorsAreDistinct guards the "no more single cyan for
// everything" requirement: each cost-share bucket must render in its own
// color, not the generic accent.
func TestCostBucketColorsAreDistinct(t *testing.T) {
	r := New(Options{NoColor: false, IsTTY: true, Dark: true})
	buckets := []string{"cache_write", "cache_read", "input", "output"}
	seen := map[string]bool{}
	for _, b := range buckets {
		c := r.CostBucketColor(b)
		key := colorKey(c)
		if seen[key] {
			t.Fatalf("CostBucketColor(%q) collides with another bucket's color (key=%s)", b, key)
		}
		seen[key] = true
	}
}
