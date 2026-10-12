# Our usage, 2026-09-11 to 2026-10-11

API-equivalent cost, computed by `ccusage --by-agent` at each model's price
(`results/ccusage_*_by_agent.json`). On a subscription this is not a bill but the order
of magnitude of the quota used. Codex read directly from the `token_count` entries in
`~/.codex/sessions` (ccusage does not see them).

| Machine | Agent | Active days | API cost | Output | Re-read (cache) | Written (cache) | Cache hit | Re-read / output |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| Server A | Hermes (3 profiles) | 29 | $1,338 | 7.3 M | 1,584 M | 143 M | 92% | 218x |
| Laptop | Claude Code 2.1.296 | 18 | $703 | 2.5 M | 1,066 M | 11.6 M | 99% | 427x |
| Laptop | Codex (gpt-5.x), full history 2025-11-15 to 2026-08-02 | 12 | not priced | 0.48 M | 107 M (cached input) | n/a | 94% | 234x |
| Server A | Claude Code 2.1.220 | 1 | $0.16 | benches of this repo | | | | |
| Server B | Hermes (Opus 5.5 / Sonnet 5, 1 h cache) | 31 | $236 (Hermes estimate) | 3.9 M | 400 M | 32.8 M | 92% | 102x |

Hermes on server A, by profile and model (`~/.hermes/state.db` and profiles, same period):

| Profile / model | Sessions | Calls | Re-read | Written | Written / re-read |
|---|---:|---:|---:|---:|---:|
| Main assistant, Opus 5.5 (long sessions) | 46 | 6,593 | 1,149 M | 95.5 M | 8% |
| Main assistant, Sonnet 5 (subagents) | 132 | 1,739 | 102 M | 18.4 M | 18% |
| Main assistant, Opus 5 | 109 | 1,536 | 182 M | 13.8 M | 8% |
| Main assistant, Fable 5.1 | 2 | 1,235 | 151 M | 15.8 M | 10% |
| Specialized agent 1, Opus 5 / 5.5 | 67 | 629 | 92 M | 7.1 M | 8% |
| Specialized agent 2, Sonnet 5 | 10 | 93 | 3 M | 0.4 M | 13% |

Server B, detail: almost everything goes through one chat-bot profile (chat, crons, and 159
subagent sessions out of 263), 6,274 calls. The other profiles are dormant or marginal ($4).

| Profile / model | Sessions | Calls | Re-read | Written | Written / re-read |
|---|---:|---:|---:|---:|---:|
| Chat bot, Sonnet 5 | 184 | 3,862 | 208 M | 19.2 M | 9% |
| Chat bot, Opus 5.5 | 55 | 1,749 | 133 M | 9.5 M | 7% |
| Chat bot, Sonnet 5.5 | 20 | 663 | 53 M | 3.2 M | 6% |
| Secondary profile, Sonnet 5 / Opus 5.5 | 30 | 223 | 6.5 M | 0.8 M | 12% |

## What it says

1. **Hermes weighs twice as much as Claude Code on the laptop**, and writes 12x more to cache
   (143 M against 11.6 M) for a comparable volume re-read. The laptop holds a 99% hit rate,
   Hermes 92%: that is the gap to attack, a write costing 40x a read with a 1 h TTL.
2. **Subagents rewrite 18% of what they re-read**, twice the rate of main sessions: a short
   subagent pays for writing its prefix without amortizing it. A subagent is only worth it if it
   makes enough calls to amortize its prefix.
3. **Claude Code on the laptop re-reads 427x what it outputs**: very long sessions. Lever: `/clear`
   between two tasks, `/compact` earlier.
4. **Two output-style plugins**: in Claude Code, their `SessionStart` hooks inject themselves in a
   non-deterministic order, which rewrites ~6.4k tokens on every new session one time in two
   (`results/cache_diff_claude_code.txt`, measured). In Hermes, one of them reinjected its 5.2k
   characters on every message, kept in history and re-read on every following call; fixed on
   2026-10-11 (body sent once, 90-character reminder after).
5. Server B costs 6x less than server A. Its main item is the chat bot's subagents (60% of
   sessions); same logic as in 2: short sessions pay for their prefix without amortizing it.
6. Codex is marginal (last session 2026-08-02).

## Applied on 2026-10-11: `prompt_caching.cache_ttl: auto`

`auto` keeps the 1 h TTL for human sessions and switches machine sessions (subagent, cron...)
to 5 min, whose write costs 1.25x instead of 2x. Estimated with `tools/ttl_auto_estimate.py`:
a machine session where no gap between messages exceeds 5 min loses nothing with the short TTL.

| Host | Machine sessions | With a gap > 5 min | Machine share of writes | Saving on writes |
|---|---:|---:|---:|---:|
| Server A | 116 | 1 | 4% | 1.5% |
| Server B | 203 | 4 | 54% | 18.7% (lower bound) |

Applied on both hosts, config backed up first. Read when each agent is created: active from
the next session, no restart.
