package cliapp

import "testing"

// TestCostWithoutCacheRepricesCacheReadsAsInput is the hand-computed
// fixture for the Summary tab's "what would this have cost without the
// cache" figure: cache_read tokens are repriced at the plain input rate
// (hermesusage.PriceUSDPerMTok["input"] = $4/MTok), input/output/
// cache_write costs are carried over unchanged from a.USD.
//
// Fixture: 2,000,000 cache-read tokens, a.USD = {input: 4.0, output: 0,
// cache_read: 0.40, cache_write: 0}.
//
//	with cache:    4.0 + 0 + 0.40 + 0 = 4.40
//	without cache: 4.0 + 0 + 0      + (2,000,000 * 4.0 / 1e6 = 8.0) = 12.0
func TestCostWithoutCacheRepricesCacheReadsAsInput(t *testing.T) {
	a := AgentCacheResult{
		CacheReadTokens: 2_000_000,
		USD:             map[string]float64{"input": 4.0, "output": 0, "cache_read": 0.40, "cache_write": 0},
	}
	got := costWithoutCache(a)
	want := 12.0
	if got != want {
		t.Fatalf("costWithoutCache = %v, want %v", got, want)
	}
}

// TestTotalCacheUSDSumsAllFourBuckets pins the shared helper cacheCard
// and the Summary tab both use for "what this cost you".
func TestTotalCacheUSDSumsAllFourBuckets(t *testing.T) {
	a := AgentCacheResult{USD: map[string]float64{"input": 1, "output": 2, "cache_read": 0.5, "cache_write": 3}}
	got := totalCacheUSD(a)
	want := 6.5
	if got != want {
		t.Fatalf("totalCacheUSD = %v, want %v", got, want)
	}
}
