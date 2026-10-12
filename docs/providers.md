# Prompt cache rules by provider

Every major provider caches the **start** of your prompt and bills cached tokens at a
fraction of the input price. They differ on four points: do you have to opt in, how long the
cache lives, whether writing to it costs extra, and how big the prompt must be before it is
cached. Checked on 2026-10-12 against the sources linked below; prices move, re-check before
you rely on a multiplier.

| Provider | Opt-in | Write | Read | Lifetime | Minimum | Usage field |
|---|---|---|---|---|---|---|
| Anthropic (Claude) | yes: `cache_control` (one top-level field, or up to 4 breakpoints) | 1.25x (5 min TTL), 2x (1 h TTL) | 0.1x | 5 min or 1 h, refreshed on each hit | per model | `cache_creation_input_tokens`, `cache_read_input_tokens` |
| OpenAI | no, automatic | free before GPT-5.6, 1.25x from GPT-5.6 | 0.25x to 0.5x | 30 min (GPT-5.6+), else ~5 to 10 min idle, up to 24 h with `24h` retention | 1,024 tokens | `cached_tokens` |
| Google Gemini (2.5+) | no, implicit; explicit `CachedContent` also available | free (implicit) | 0.25x | ~3 to 5 min (implicit) | 1,024 to 4,096 tokens depending on model | `usageMetadata.cachedContentTokenCount` |
| DeepSeek | no, automatic | 1x | 0.1x | not documented, best effort | not documented | `prompt_cache_hit_tokens`, `prompt_cache_miss_tokens` |
| xAI Grok, Moonshot | no, automatic | free | 0.25x | not documented | not documented | `cached_tokens` |
| Groq | no, automatic | free | 0.5x | not documented | not documented | `cached_tokens` |

Through **OpenRouter**, the same rules apply per underlying provider, and requests are pinned to
the provider that served the previous cached call ("sticky routing") so hits survive. Its
`cache_discount` field gives the saving per request.

## What this means in practice

- **Same rule everywhere: the cache is a prefix match.** One byte that changes near the start
  (a timestamp, a reordered tool list, a different model) and everything after it is billed
  again. This is the main lever on every provider.
- **Anthropic** is the only one where you can forget to turn caching on. Coding agents built on
  it (Claude Code, Hermes, Aider with `--cache-prompts`) do it for you; your own code
  must set `cache_control`.
- **Write premiums** (Anthropic, OpenAI GPT-5.6+, DeepSeek at full price) make a broken cache
  cost *more* than no cache at all. Automatic caches with free writes (Gemini implicit, Grok)
  only lose the discount.
- **Short lifetimes** (Gemini ~3 to 5 min, Anthropic 5 min default) mean a pause longer than
  that starts cold. A human typing slowly or a cron every 10 minutes pays the write each time.
- **Below the minimum**, nothing is cached. Tiny prompts do not benefit; big system prompts and
  long sessions do.

## Sources

- Anthropic, OpenAI, Gemini, DeepSeek, Grok, Moonshot, Groq multipliers:
  [OpenRouter, prompt caching guide](https://openrouter.ai/docs/guides/best-practices/prompt-caching)
- OpenAI lifetimes and `prompt_cache_key`:
  [OpenAI, prompt caching](https://platform.openai.com/docs/guides/prompt-caching)
- Gemini implicit caching and minimums:
  [Google, context caching](https://ai.google.dev/gemini-api/docs/caching)
- DeepSeek usage fields: [DeepSeek, KV cache](https://api-docs.deepseek.com/guides/kv_cache)
- Aider `--cache-prompts` and `--cache-keepalive-pings`:
  [Aider, prompt caching](https://aider.chat/docs/usage/caching.html)
