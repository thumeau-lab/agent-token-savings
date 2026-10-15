<p align="center">
  <img src="docs/img/ats-dashboard.png" alt="ats dashboard: cost, money saved by the cache, cache hit rate, efficiency score, activity heatmap, daily spend, what to improve" width="100%">
</p>

<p align="center">
  <b>See where your AI coding agent's prompt cache saves money, where it leaks, and what to change.</b><br>
  <code>ats</code> is one offline binary that reads your agents' local logs. Plus the methods and measurements behind it.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-555555?style=flat-square" alt="MIT license"></a>
  <img src="https://img.shields.io/badge/ats-Go_single_binary-00add8?style=flat-square&logo=go&logoColor=white" alt="Go single binary">
  <img src="https://img.shields.io/badge/offline-no_telemetry-1b1b19?style=flat-square" alt="Offline, no telemetry">
  <img src="https://img.shields.io/badge/reads-Claude_Code_%7C_Codex_%7C_Copilot_CLI_%7C_Hermes-1b1b19?style=flat-square" alt="Reads Claude Code, Codex, Copilot CLI, Hermes">
</p>

<p align="center">English | <a href="README.fr.md">Français</a></p>

## `ats` in one minute

```sh
go install github.com/thumeau-lab/agent-token-savings/cli/cmd/ats@latest
ats
```

Prebuilt binaries (macOS, Linux, Windows, amd64 and arm64) are on the
[releases page](https://github.com/thumeau-lab/agent-token-savings/releases); `install.sh`
fetches the right one, checks its SHA256 and puts it in `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/thumeau-lab/agent-token-savings/main/install.sh | sh
```

`ats` with no argument opens a full-screen dashboard. It detects the agents installed on the
machine and reads their local session logs. Nothing leaves your machine: no account, no API
key, no network at runtime.

| Agent | What `ats` reads |
|---|---|
| Claude Code | `~/.claude/projects/*/*.jsonl` |
| Codex CLI | `$CODEX_HOME/sessions` (default `~/.codex`) |
| GitHub Copilot CLI | `~/.copilot/session-store.db`, read-only |
| Hermes Agent | `~/.hermes/state.db` and its profiles |

### What you see

Five tabs, `1` to `5` or `tab` to switch:

1. **Dashboard.** What you spent, what the cache saved you, your cache hit rate, in big digits.
   Under them: yesterday, 7-day average and the change since yesterday. Around them: an
   efficiency score out of 100 with a letter, an activity heatmap, daily spend, cache hit rate
   per agent and per model, a ranked **To improve** list, and a **Measured** panel that says
   exactly which files each number comes from and what `ats` could not measure.
2. **Costs.** Cache reads, cache writes, input and output per agent, and the abnormal cache
   rebuilds: the calls where the whole prompt was written again.
3. **What gets sent.** What fills the start of the prompt (system prompt, tool schemas,
   skills) and is paid again on every call.
4. **Cache lifetime.** Whether the cache TTL fits how you work: long for a human who answers
   slowly, short for sub-agents and crons.
5. **Fix.** The settings worth changing, as a diff. Nothing is written until you confirm, and
   a timestamped `.bak` is kept. `ats` never proposes disabling the cache.

The left and right arrow keys change the period, `p` the agent, `L` the language (English, French), `?` explains
every number, including the efficiency score formula.

### In scripts

```text
ats cache  [agent] [--days N] [--json]   cache hit rate, reads, writes, rebuilds
ats prefix [agent] [--json]              what the prompt prefix is made of
ats ttl    [agent] [--days N] [--json]   does the cache lifetime fit your sessions
ats fix    [--write]                     show the fix, apply it with --write
```

Piped or with `--no-tui`, `ats` prints plain text; `--json` is stable and always in English.
`NO_COLOR` is honoured, `--lang fr` or `ATS_LANG=fr` switches the language.

Dollar amounts use public API prices. On a subscription they measure the share of quota you
used, not money you paid. When `ats` does not know a model's price (Codex, Copilot), it says so
instead of guessing. For overall spend across providers, see
[CodeBurn](https://github.com/getagentseal/codeburn); `ats` only looks at the cache.

## Why your agent burns tokens

An agent sends **the whole conversation** back to the model on every message. You think you
pay for what the model writes; you mostly pay for what it re-reads, and even more for what it
has to re-write when its cache breaks. Over 21 days on our setup, the model's answers were
9% of the bill. Cache writes were 71%.

Three ideas cover most of the savings, whatever the tool or the model:

1. **Don't break the cache.** The start of the prompt must never change.
2. **Keep sessions short.** One session per task, compact earlier.
3. **Load less up front.** Every tool, skill or MCP server loaded is paid again on every message.

Want your agent to apply them for you? Paste this into it. It measures, proposes each change
before making it, then shows you the before / after:

```text
Read https://raw.githubusercontent.com/thumeau-lab/agent-token-savings/main/SETUP.md and apply it to my setup.
```

## How the cache works

Every major provider (Anthropic, OpenAI, Google, DeepSeek...) caches the **start** of the
prompt. Each turn, the part already seen is re-read from cache for a fraction of the price
(green); only the new tail is written (orange). Change one byte near the start, and everything
is written again.

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/img/cache-timeline-dark.svg"><img src="docs/img/cache-timeline.svg" alt="7 turns: the cached prefix grows, a change at turn 6 forces a full rewrite" width="100%"></picture></p>

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/img/prompt-order-dark.svg"><img src="docs/img/prompt-order.svg" alt="Before: the date at the start forces a full rewrite. After: stable start re-read from cache, only the tail written." width="100%"></picture></p>

Rules differ per provider (opt-in, lifetime, write premium, minimum size): see
[`docs/providers.md`](docs/providers.md).

## The methods

<p align="center"><picture><source media="(prefers-color-scheme: dark)" srcset="docs/img/hero-dark.svg"><img src="docs/img/hero.svg" alt="71% of the bill is cache writes, 68% of the prefix is tool schemas, -61% git log output through rtk" width="100%"></picture></p>

| Method | How | Measured gain |
|---|---|---|
| Stable prefix | nothing timestamped or random at the start of the prompt; don't switch model, reasoning effort or MCP servers mid-session | cache writes were 71% of the bill |
| Cache lifetime that fits | long TTL when a human replies slowly, short for sub-agents and crons | -18.7% writes on a cron-heavy machine |
| Short sessions | new session per task, compaction at ~150k tokens instead of half the window | -58% context re-read per call (one agent, correlation) |
| Fewer tools loaded | turn off MCP servers, skills and tools you can't use on this machine | tool schemas were 68% of one agent's prefix |
| Cheaper model for small work | sub-agents, summaries and titles on a small model | per-token price (provider docs) |
| Shell output, verbosity | output filters such as [RTK](https://github.com/rtk-ai/rtk), shorter answers | -43 to -61% bytes on `git`, but a minor item |

## Commands per tool

| Tool | New session | Compact | See what fills the context | Cache |
|---|---|---|---|---|
| Claude Code | `/clear` | `/compact <focus>` | `/context`, `/mcp` | automatic |
| Codex CLI | `/new` | `/compact` | `/status`, `/mcp` | automatic (OpenAI) |
| Gemini CLI | `/clear` | `/compress` | `/stats`, `/memory list`, `/mcp` | implicit (Gemini 2.5+) |
| OpenCode | `/new` | `/compact` | | depends on provider |
| Aider | `/clear` | | `/tokens` | `--cache-prompts`, `--cache-keepalive-pings N` |
| Cursor | new chat | | | provider side |
| Hermes Agent | `/new` | `compression.threshold_tokens` | `hermes prompt-size` | `prompt_caching.cache_ttl` |
| Your own code | | | `usage` in each response | see [`providers.md`](docs/providers.md) |

Details, sources and evidence level of each lever: [`docs/levers.md`](docs/levers.md). All our
numbers: [`docs/measurements.md`](docs/measurements.md).

## Without `ats`

Python 3, standard library only. The scripts only read; default paths are those of a standard
install.

```
npx ccusage daily
python3 tools/claude_cache_ratio.py
python3 tools/hermes_usage.py --days 14
```

Watch tokens **re-read and written per call**, not only the cache hit rate: it stays around
90% even when the bill is high. To find which part of the prompt breaks the cache,
`tools/cache_diff.py` captures two requests through a local proxy and names the first block
that differs (Anthropic-compatible clients). More in
[`docs/measurements.md`](docs/measurements.md#reproduce).

## Build from source

```sh
git clone https://github.com/thumeau-lab/agent-token-savings
cd agent-token-savings/cli
go test ./...
go build -o ats ./cmd/ats
```

## Limits

One user, our machines, mostly Anthropic models. Before / after is a correlation, not a
controlled experiment. The methods are general; your numbers will differ, which is why `ats`
and the scripts are here. The screenshot above uses synthetic data.

## License

MIT, see [`LICENSE`](LICENSE).
