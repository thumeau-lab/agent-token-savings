# Lever catalog

Status: **measured** = figure produced by a script in this repo (see `results/`).
**read** = verified in official docs on 2026-10-11. **reported** = found by
web search, not re-verified here. Vendor claims remain claims.

This catalog comes from our own setup (Anthropic models via Claude Code and Hermes Agent). Cache rules for OpenAI, Gemini, DeepSeek and others: [`providers.md`](providers.md).

Order of magnitude to keep in mind (measured, `results/hermes_usage.txt`): over 21
days of Hermes usage, `cache_read` weighs **227x** the output in tokens. Valued at
Opus 5.5 API pricing (a write costs 40x the price of a read), cache write still
makes up **71%** of the bill, read 20%, output 9%. Priority: do not
break the cache, then reduce re-read context, then the rest.

## 1. Anthropic API

| Lever | Effect | Status | Source |
|---|---|---|---|
| Prompt caching (`cache_control`, 4 breakpoints max) | read at 0.1x the input price (0.05x Opus/Sonnet 5.5, 0.025x Fable/Mythos 5.1) | read | [prompt-caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) |
| 1h TTL instead of 5 min | write 2x instead of 1.25x; worthwhile if turns are more than 5 min apart | read | idem |
| Automatic caching (one root `cache_control`) | the breakpoint follows the end of the conversation on its own | read | idem |
| Minimum cacheable size | 512 tokens on 5.x models; below that, no error but no cache | read | idem |
| `tools` then `system` then `messages` order | changing an invalid tool invalidates everything; `tool_choice`, images, thinking, `effort` only invalidate `messages` | read | idem |
| Cache and rate limits | `cache_read_input_tokens` does not count toward ITPM for most models | read | [rate-limits](https://platform.claude.com/docs/en/api/rate-limits) |
| Message Batches | -50% on all tokens, stackable with cache; results within 24h (often < 1h); no `max_tokens: 0` warm-up in a batch | read | [batch-processing](https://platform.claude.com/docs/en/build-with-claude/batch-processing) |
| Token counting | free (rate-limited): estimate before sending, route the model | read | [token-counting](https://platform.claude.com/docs/en/build-with-claude/token-counting) |
| Context editing `clear_tool_uses_20250919` (beta `context-management-2025-06-27`) | server-side clearing of old tool results; **invalidates the cache** on each pass, tune `clear_at_least` so it's worth the rewrite | read | [context-editing](https://platform.claude.com/docs/en/build-with-claude/context-editing) |
| Tool search + `defer_loading: true` | schemas loaded on demand; ~55k tokens of MCP definitions cut by more than 85% per the docs | read (vendor claim) | [tool-search-tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool) |
| Stable thinking budget / `effort` | less output; changing it mid-conversation breaks the cache | reported | [extended-thinking](https://docs.claude.com/en/docs/build-with-claude/extended-thinking) |

## 2. Claude Code (CLI 2.1.220)

Bench: `bench/claude_code_levers.py`, Haiku, empty cwd, `claude -p` twice per variant.
Default prefix: **28,362 tokens** before the user's first word.

| Lever | Measured prefix | Status |
|---|---|---|
| `--tools ""` (no built-in tools) | 9,072 (-68%): tool schemas are the bulk of the prefix | measured |
| `--setting-sources project` (ignores `~/.claude`, custom plugins and hooks) | 24,311 (-14%) | measured |
| `--disable-slash-commands` (no skills) | 25,741 (-9%) | measured |
| `--exclude-dynamic-system-prompt-sections` | 28,168 (-0.7%), but 1st call recreates 6.7-7.5k instead of 10.6k | measured |
| `--strict-mcp-config` | 0: no MCP configured on this machine | measured |
| `DISABLE_PROMPT_CACHING=1` | same size, **0 cache read**: everything paid at full price. Never enable | measured |
| Cache TTL | `ephemeral_1h` measured here (subscription). Docs: 1h only on subscription within the included quota; on API key or usage credits, 5 min unless `promptCacheTtl: "1h"` / `CLAUDE_CODE_PROMPT_CACHE_TTL` (v2.1.242+) | measured + read, [prompt-caching](https://code.claude.com/docs/en/prompt-caching) |
| What invalidates the cache in-session | changing model, effort, connecting an MCP, denying a whole tool via a `deny` rule without tool search, a Claude Code update. Do NOT invalidate it: editing `CLAUDE.md` (picked up on next `/clear`), skills, plan mode, `/rewind` | read, idem |
| `/usage`: `Prompt cache (main)` line | hit ratio, miss count, likely cause of the last miss (v2.1.260+) | read, idem |
| Cache scope | one machine and one folder; two successive sessions only share the prefix if the startup `git status` snapshot is identical | read, idem |
| `/clear` between topics, `/compact <focus>` before auto-compact | context reset or summarized | reported, [costs](https://code.claude.com/docs/en/costs) |
| `/context`, `/cost`, statusline | see what's heavy before acting | reported, idem |
| `/mcp`: turn off unneeded servers; MCP tool search active by default | fewer schemas at startup | reported, idem |
| Subagents | isolated context, only the summary comes back | reported, [context-window](https://code.claude.com/docs/en/context-window) |
| `/model`: Haiku for trivial work, `opusplan` | price per token | reported, [costs](https://code.claude.com/docs/en/costs) |
| `MAX_THINKING_TOKENS`, `CLAUDE_CODE_MAX_OUTPUT_TOKENS`, `MAX_MCP_OUTPUT_TOKENS` | cap thinking, response, MCP output | reported, [model-config](https://code.claude.com/docs/en/model-config) |
| Short `CLAUDE.md` (< 200 lines), rest in skills | loaded every session; `@imports` too | reported, [features-overview](https://code.claude.com/docs/en/features-overview) |
| `PreToolUse` hook (deny or `updatedInput`) | blocks a noisy read or command before execution | reported, [hooks](https://code.claude.com/docs/en/hooks) |
| `permissions.deny` on large files | no official `.claudeignore`; reliability debated ([#18846](https://github.com/anthropics/claude-code/issues/18846)) | reported |

Noise observed: on `default`, the 2nd identical call sometimes recreates ~6.5k (2 runs out of 3).
Unverified hypothesis: docs say the prefix carries a snapshot of the folder
(git status, platform, memory paths) taken at startup; one section that varies
between two launches is enough to rewrite the rest. `/usage` in v2.1.260+ would show the cause.

## 3. Hermes Agent

Measured with `tools/hermes_usage.py --split 2026-10-03` (date of the tuning batch):
cache_read per API call **241,521 before, 101,451 after (-58%)**, hit rate stable at 92%.
This is a before/after correlation, not a controlled experiment: the nature of the work
also changed (one session of 1,220 calls at 398k per call dominates the "before").

| Setting (`~/.hermes/config.yaml`) | Value in place | Effect |
|---|---|---|
| `prompt_caching.cache_ttl` | `auto` | 1h for human sessions, 5 min for subagents, crons, kanban; `tools/ttl_auto_estimate.py` scores the gain per host before switching |
| `compression.threshold_tokens` | `150000` | compaction before each turn costs 200k+ |
| `compression.proactive_prune_tokens` | `100000` | prunes large tool results |
| `compression.idle_compact_after_seconds` | `3600` | compacts a session resumed after a break (the cache is cold anyway) |
| `auxiliary.*` | Sonnet | summaries, titles, background review outside Opus |
| `delegation.model` | Sonnet | leafs only feed back their summary |
| `/new` as soon as a topic is closed | habit | lever #1: cache_read = turns x context size |

Settings docs: <https://hermes-agent.nousresearch.com/docs> (reported by the research leaf).
`ccusage hermes daily` reads the same sessions: over the 21 days, a +0.6 to +0.9% gap vs
`tools/hermes_usage.py` on read, write and output (measured, `results/ccusage_hermes.json`).
Its cost ($1,289) differs from ours because it applies the price of each model actually used.

### What goes out with every message

`hermes prompt-size` shows the last three blocks, not the first: it travels in the
user message, not in the system prompt.

| Block | Measure | Lever | Measured here |
|---|---|---|---|
| Per-message injection (memory recall, `pre_llm_call` plugins): appended to the user message, **kept in history** and re-read on every following turn | SQL query below | memory recall size (Hindsight `recall_max_tokens`, default 4096); style plugins: full body once per session, one-line reminder after | ~28k characters per message (memory 17k, ponytail 5.2k, caveman 3.1k); ponytail 5,228 down to 90 after the fix |
| Skills index (`name + description` of each active skill) | `hermes prompt-size` | `skills.disabled` list in `config.yaml` (reversible; `hermes skills opt-out --remove` deletes files); criterion: incapable on this host, not "rarely used" | 9.3 KB, 30% of the system prompt before pruning; -3.7 KB per message after |
| Tool schemas | `hermes prompt-size --platform <p> --json` (`toolsets_breakdown`) | disable / measure / re-enable a toolset in the same loop; tools deferred behind `tool_search` (zero standing cost) | 68% of the prefix; the 5 heaviest are also the most used: verdict "nothing to cut" |
| Persistent memory | sort entries by **length** | compact into "trigger + conclusion", do not raise `memory_char_limit` | 22 to 24 entries served, same cap |

```sql
-- ~/.hermes/state.db: characters injected on average per user message, last 3 days
SELECT count(*), sum(length(api_content) - length(content)) / count(*)
FROM messages WHERE role = 'user' AND api_content IS NOT NULL
  AND timestamp > unixepoch('now', '-3 days');
```

Not worth it: condensing the identity file (`SOUL.md`). Measured: 63 net tokens
on a 17.8k prefix after rereading, and six behavior rules weakened in the
process that had to be restored.

## 4. Community tools

| Tool | What it reduces | Status |
|---|---|---|
| [RTK](https://github.com/rtk-ai/rtk) | shell output only. Measured here: `git status` -43%, `git log` -61%, `ls -la` -61%, `git diff --stat` 0%. `find` goes from 320 KB to 1 KB **by truncating** (`+7389 more`): real gain but lost info | measured (bytes, not tokens) |
| Terse output-style plugins | agent output style; output weighs ~0.4% of cache_read here, so marginal gain on the bill | reasoned from our own numbers |
| [ccusage](https://github.com/ccusage/ccusage) | nothing, it measures (local, read-only); matches our Hermes numbers within 1% | measured |
| [claude-code-router](https://github.com/musistudio/claude-code-router) | routes to a cheaper model; "10x" = claim | reported |
| [Serena](https://github.com/oraios/serena) | per-symbol reads (LSP) instead of the whole file; no independent figure | reported |
| [Context7](https://github.com/upstash/context7) | targeted docs instead of full docs or invented API | reported |
| [repomix](https://github.com/yamadashy/repomix) | one repo pack instead of N reads; useful one-shot, counterproductive in a long session (the whole pack re-read every turn) | reported |
| [LLMLingua](https://github.com/microsoft/LLMLingua) | algorithmic prompt compression, up to 20x on GSM8K (paper); breaks the cache if the compressed prompt varies | reported |

## Recommended order of action

1. Measure (`tools/hermes_usage.py`, `tools/claude_cache_ratio.py`).
2. Short sessions: `/new` or `/clear` per topic. This is the dominant term.
3. Heavy work in a subagent, cheaper model for leafs and auxiliaries.
4. Stable, short prefix: fewer tools, skills, MCPs; nothing timestamped in the system prompt.
5. Automatic compression and pruning as a safety net.
6. Only after that: RTK, terse output style, reasoning effort.
