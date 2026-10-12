# SETUP.md: instructions for an agent

This file is for you, the coding agent (Claude Code, Codex, Gemini CLI, Cursor, Aider,
OpenCode, Hermes Agent, or a program calling an LLM API). Your user wants their setup to use
fewer tokens. Follow the steps in order.

Rules, no exceptions:

- **Measure before changing** anything, and keep the numbers for the end.
- **Show each change** (file, old value, new value) and wait for the user's approval before
  writing. Back up the file first (`cp x x.bak`).
- **Never turn prompt caching off**, not even "to test".
- Never read, print or copy an API key or token.
- One lever at a time when possible, so you know which one worked.

## 1. Identify the setup

Find which agent(s) and which model provider are in use. Several can coexist: handle each.

| Found | Agent |
|---|---|
| `~/.claude/projects/` | Claude Code |
| `~/.codex/` | Codex CLI |
| `~/.gemini/` | Gemini CLI |
| `.aider.conf.yml` or `aider` on the PATH | Aider |
| `~/.hermes/config.yaml` | Hermes Agent |
| code calling `messages.create`, `chat.completions.create`, `responses.create` or `generateContent` | direct API use |

Then read the provider's cache rules in
[`docs/providers.md`](https://github.com/thumeau-lab/agent-token-savings/blob/main/docs/providers.md):
whether caching is opt-in, how long it lives, whether writes cost extra.

## 2. Measure the baseline

For coding CLIs:

```
npx ccusage daily
```

For Claude Code or Hermes, more detail (Python 3 stdlib, read-only):

```
git clone --depth 1 https://github.com/thumeau-lab/agent-token-savings /tmp/ats
python3 /tmp/ats/tools/claude_cache_ratio.py
python3 /tmp/ats/tools/hermes_usage.py --days 14
```

For direct API use: log the usage fields of a few typical requests (field names per provider
in `providers.md`).

Write down two numbers: **tokens read from cache per call** and **tokens written (or uncached
input) per call**. Ignore the cache hit rate: it stays around 90% even on an expensive setup.
Then pick:

| Symptom | Likely cause | Step |
|---|---|---|
| written or uncached per call > 20k | the start of the prompt changes, cache rewritten | 3a |
| read per call > 150k | sessions too long | 3b |
| a fresh session already starts > 25k | too many tools, skills or MCP servers loaded | 3c |
| none of these | already healthy, marginal gains only | 3d |

Start with the highest row that applies: on Anthropic, a cache write costs 12 to 20 times a read.

## 3a. Don't break the cache

Caches match a prefix: tools, then system prompt, then messages, in that order. One changed byte
invalidates everything after it.

For everyone:

- Nothing timestamped, random or recomputed in the system prompt or in instruction files
  (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, Cursor rules, Aider conventions...).
- Don't switch model, reasoning effort, or plug/unplug an MCP server mid-session.
- Content that must vary (date, memory recall, retrieved docs) goes at the end, in the user
  message.
- Several startup hooks injecting text can run in a non-deterministic order: keep as few as
  possible.

Per tool:

- **Claude Code**: `/usage` shows the probable cause of the last cache miss. With an API key the
  default TTL is 5 min; if turns are more than 5 min apart, set `"promptCacheTtl": "1h"` in
  `settings.json`.
- **Aider**: run with `--cache-prompts`. Add `--cache-keepalive-pings N` only if the user pauses
  longer than 5 min between messages.
- **Hermes Agent**: `hermes config set prompt_caching.cache_ttl auto` (1 h for human sessions,
  5 min for sub-agents and crons). Estimate first:
  `python3 /tmp/ats/tools/ttl_auto_estimate.py ~/.hermes`.
- **Codex CLI, Gemini CLI, Cursor, OpenCode**: caching is handled by the tool or the provider;
  the general rules above are what you control.

Direct API:

- **Anthropic**: caching is opt-in. Put `cache_control` on the last stable block:

  ```python
  resp = client.messages.create(
      model=MODEL, max_tokens=1024, messages=history,
      system=[{"type": "text", "text": STABLE_SYSTEM_PROMPT,
               "cache_control": {"type": "ephemeral"}}],
  )
  print(resp.usage.cache_creation_input_tokens, resp.usage.cache_read_input_tokens)
  ```

  Add `"ttl": "1h"` only if calls are more than 5 min apart: a 1 h write costs 2x input against
  1.25x for 5 min.
- **OpenAI**: automatic above 1,024 tokens. Send a stable `prompt_cache_key` per workflow so
  similar requests land on the same cache.
- **Gemini**: implicit on 2.5+ models; keep the shared content at the start. For a large
  document reused many times, consider explicit `CachedContent`.
- **DeepSeek, Grok, others**: automatic; only the prefix rule matters.

Finding the block that changes (clients honoring `ANTHROPIC_BASE_URL`): the proxy forwards to
`api.anthropic.com` and records request bodies only, never headers.

```
python3 /tmp/ats/tools/cache_diff.py proxy /tmp/cap
```

In another terminal, run the same command twice:

```
ANTHROPIC_BASE_URL=http://127.0.0.1:8977 claude -p "ok"
```

```
python3 /tmp/ats/tools/cache_diff.py diff /tmp/cap/000.json /tmp/cap/001.json
```

The output names the first block that differs: that is the one to freeze. For other clients,
dump two consecutive request bodies and diff them the same way.

## 3b. Shorten sessions

The context is re-read on every call: a 300k-token session costs 300k of reads per message.

- Tell the user to start a new session per task: `/clear` (Claude Code, Gemini CLI, Aider),
  `/new` (Codex, OpenCode, Hermes), a new chat (Cursor).
- Compact before the automatic limit: `/compact <focus>` (Claude Code), `/compact` (Codex,
  OpenCode), `/compress` (Gemini CLI).
- Large exploration (reading many files, searching) in a sub-agent: only the summary comes back.
- **Hermes Agent**:
  ```
  hermes config set compression.threshold_tokens 150000
  ```
  ```
  hermes config set compression.proactive_prune_tokens 100000
  ```
  ```
  hermes config set compression.idle_compact_after_seconds 3600
  ```
  Absolute compaction ceiling (without it, a 1M-context model compacts around 500k), LLM-free
  pruning of old tool results, compaction of a session resumed after a pause (the cache is cold
  anyway).
- **Direct API, Anthropic**: context editing (`clear_tool_uses_20250919`) drops old tool results
  server-side; it invalidates the cache from the cleared point.

## 3c. Load less up front

Every tool schema, skill index entry or MCP server is paid again on every call. Cut criterion:
**unusable on this machine** (no account, no GPU, outside the user's work), not "rarely used".

- **Claude Code**: `/context` shows what weighs; `/mcp` to disable unused servers; keep
  `CLAUDE.md` under 200 lines, move the rest into skills loaded on demand.
- **Codex CLI**: `/mcp` lists servers; remove unused ones from `~/.codex/config.toml`; keep
  `AGENTS.md` short.
- **Gemini CLI**: `/mcp` and `/memory list` show what is loaded; trim `GEMINI.md` files.
- **Aider**: `/tokens` shows the context; `/drop` files no longer needed.
- **Cursor**: disable unused MCP servers in settings; @-mention only the files needed.
- **Hermes Agent**: `hermes prompt-size --platform cli --json` gives `toolsets_breakdown`. To
  measure one toolset: `hermes tools disable <name>`, measure, `hermes tools enable <name>`, check
  the total is back. Skills: add them to `skills.disabled` in `config.yaml` (reversible); never
  `hermes skills opt-out --remove`, which deletes files.
- **Direct API**: send only the tools the task needs; Anthropic tool search with
  `defer_loading: true` loads schemas on demand.

## 3d. Marginal gains

- Cheaper model for simple work: sub-agents, summaries, titles, commit messages.
- Shell output: [RTK](https://github.com/rtk-ai/rtk) cuts `git status` / `git log` output by
  43 to 61%, but truncates `find` enough to hide the list.
- Shorter answers: model output was 9% of the bill on our setup.

## 4. Verify

After 3 to 7 days of normal use, run the same measurement again. For Hermes, `--split` compares
the two periods:

```
python3 /tmp/ats/tools/hermes_usage.py --days 14 --split YYYY-MM-DD
```

`YYYY-MM-DD` = day of the change. Give the user read per call and written per call, before and
after. If writes per call went up, revert the last change from its `.bak`.

## Never do

- Disable prompt caching (`DISABLE_PROMPT_CACHING=1` or equivalent): everything is billed at
  full price, measured 1.8x more expensive per call.
- Compress the prompt with a tool that produces different text each time (LLMLingua and the
  like): every variation rewrites the cache.
- Pack the whole repository into a long session (repomix and the like): the pack is re-read on
  every turn.
- Claim a saving without measuring it again.

Sources and evidence level of each lever:
[`docs/levers.md`](https://github.com/thumeau-lab/agent-token-savings/blob/main/docs/levers.md).
Our numbers:
[`docs/measurements.md`](https://github.com/thumeau-lab/agent-token-savings/blob/main/docs/measurements.md).
