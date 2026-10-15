package cliapp

// openAIModelPrice is one OpenAI model's list price in $/MTok (input,
// cached input, output). Cache writes are usually not billed separately
// on OpenAI's API (cached tokens are billed at a reduced input rate on
// read, with no extra charge to write them into the cache), but some
// pages explicitly state a cache-write surcharge instead ("Cache writes
// are billed at 1.25x the uncached input token rate", e.g.
// gpt-5.6-terra and gpt-5.6-luna): CacheWriteMultiplier carries that
// rate (applied to Input, not to CachedInput) when the page states one,
// and is 0 -- never guessed -- when the page says nothing about cache
// writes.
type openAIModelPrice struct {
	Input, CachedInput, Output float64 // $/MTok
	// CacheWriteMultiplier, when non-zero, is read from the model's own
	// page ("Cache writes are billed at Nx the uncached input token
	// rate"); cache-write cost is then CacheWriteTokens * Input *
	// CacheWriteMultiplier. 0 when the page states nothing about cache
	// writes (the common case).
	CacheWriteMultiplier float64
}

// openAIPriceUSDPerMTok are OpenAI's list prices in $/MTok, standard
// (non-batch, non-flex, non-priority) tier, Responses API. Only models
// actually found on OpenAI's own pages are listed -- never a guessed
// number (the card).
//
// Source (read 2026-10-16):
//
//	https://developers.openai.com/api/docs/models/gpt-5
//	https://developers.openai.com/api/docs/models/gpt-5-codex
//	https://developers.openai.com/api/docs/models/gpt-5.3-codex
//	https://developers.openai.com/api/docs/models/codex-mini-latest
//	https://developers.openai.com/api/docs/models/gpt-5.1-codex-max
//	https://developers.openai.com/api/docs/models/gpt-5.1-codex
//	https://developers.openai.com/api/docs/models/gpt-5.4
//	https://developers.openai.com/api/docs/models/gpt-5.6-terra
//	https://developers.openai.com/api/docs/models/gpt-5.6-luna
//	https://developers.openai.com/api/docs/models/gpt-5.4-mini
//	https://developers.openai.com/api/docs/models/gpt-5.5
//	https://developers.openai.com/api/docs/models/gpt-5.2-codex
//
// (openai.com/api/pricing and platform.openai.com/docs/pricing both
// return the price data via client-side JavaScript and could not be
// read directly; the per-model pages above are server-rendered and
// show the same "Input / Cached input / Output" price card.)
//
// ponytail: every page above also states a long-context surcharge
// (">272K input tokens priced at 2x input and 1.5x output for the full
// request/session") that is deliberately not modeled here -- cost is
// slightly under-reported on prompts over that 272K-token ceiling.
var openAIPriceUSDPerMTok = map[string]openAIModelPrice{
	"gpt-5":             {Input: 1.25, CachedInput: 0.125, Output: 10.00},
	"gpt-5-codex":       {Input: 1.25, CachedInput: 0.125, Output: 10.00},
	"gpt-5.3-codex":     {Input: 1.75, CachedInput: 0.175, Output: 14.00},
	"codex-mini-latest": {Input: 1.50, CachedInput: 0.375, Output: 6.00},
	"gpt-5.1-codex-max": {Input: 1.25, CachedInput: 0.125, Output: 10.00},
	"gpt-5.1-codex":     {Input: 1.25, CachedInput: 0.125, Output: 10.00},
	"gpt-5.4":           {Input: 2.50, CachedInput: 0.25, Output: 15.00},
	"gpt-5.6-terra":     {Input: 2.00, CachedInput: 0.20, Output: 12.00, CacheWriteMultiplier: 1.25},
	"gpt-5.6-luna":      {Input: 0.20, CachedInput: 0.02, Output: 1.20, CacheWriteMultiplier: 1.25},
	"gpt-5.4-mini":      {Input: 0.75, CachedInput: 0.075, Output: 4.50},
	"gpt-5.5":           {Input: 5.00, CachedInput: 0.50, Output: 30.00},
	"gpt-5.2-codex":     {Input: 1.75, CachedInput: 0.175, Output: 14.00},
}

// openAICostForModels prices a window's usage per model against
// openAIPriceUSDPerMTok and sums into the same usd/share category
// shape usdSplit returns (input/output/cache_read/cache_write).
//
// total* must be the window's real totals and models its per-model
// breakdown (ModelUsageJSON rows with Model != ""): if any model in
// models has no openAIPriceUSDPerMTok entry, or the breakdown's token
// counts do not add up to total* (some tokens belong to no known model
// at all -- Model == "" rows are excluded from the breakdown upstream),
// costUnknown is true and usd/share are both nil. Pricing only part of
// the window would silently under-report its real cost -- the card:
// never invent a $ figure.
func openAICostForModels(totalInput, totalOutput, totalCacheRead, totalCacheWrite int64, models []ModelUsageJSON) (usd, share map[string]float64, costUnknown bool) {
	if len(models) == 0 {
		return nil, nil, true
	}
	usd = map[string]float64{"input": 0, "output": 0, "cache_read": 0, "cache_write": 0}
	var coveredInput, coveredOutput, coveredCacheRead, coveredCacheWrite int64
	for _, m := range models {
		price, ok := openAIPriceUSDPerMTok[m.Model]
		if !ok {
			return nil, nil, true
		}
		coveredInput += m.InputTokens
		coveredOutput += m.OutputTokens
		coveredCacheRead += m.CacheReadTokens
		coveredCacheWrite += m.CacheWriteTokens
		usd["input"] += float64(m.InputTokens) * price.Input / 1e6
		usd["output"] += float64(m.OutputTokens) * price.Output / 1e6
		usd["cache_read"] += float64(m.CacheReadTokens) * price.CachedInput / 1e6
		// usd["cache_write"] stays 0 for models with no CacheWriteMultiplier
		// (the common case); see openAIModelPrice's doc comment.
		usd["cache_write"] += float64(m.CacheWriteTokens) * price.Input * price.CacheWriteMultiplier / 1e6
	}
	if coveredInput != totalInput || coveredOutput != totalOutput ||
		coveredCacheRead != totalCacheRead || coveredCacheWrite != totalCacheWrite {
		return nil, nil, true
	}

	var total float64
	for _, v := range usd {
		total += v
	}
	share = map[string]float64{}
	if total != 0 {
		for k, v := range usd {
			share[k] = v / total
		}
	}
	return usd, share, false
}
