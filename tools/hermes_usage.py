#!/usr/bin/env python3
"""Where the Hermes quota goes: aggregate of ~/.hermes/state.db (read-only).

    python3 tools/hermes_usage.py [--db PATH] [--days 14] [--split YYYY-MM-DD] [--json F]

Two indicators per API call:
- read/call  = average size of the context re-read. Goes down with /new, compaction, delegation.
- write/call = what is (re)written to cache. A normal turn only adds its tail
  (a few k); 30k+ per call = cache broken or rebuilt every turn.
The weighted cost uses Opus 5.5 API prices (1h cache): in tokens, reads
dominate; in dollars, it is often writes.
--split compares two periods (before/after a config change).
"""
import argparse
import json
import os
import sqlite3
import sys

# $/MTok, Opus 5.5, cache write with 1h TTL.
# https://platform.claude.com/docs/en/build-with-claude/prompt-caching#pricing (read 2026-10-11)
PRICE = {"input": 4.0, "output": 20.0, "cache_write": 8.0, "cache_read": 0.20}

DAILY = """
SELECT date(started_at,'unixepoch','localtime') d, count(*) n,
       sum(api_call_count) calls, sum(output_tokens) out_t, sum(input_tokens) in_t,
       sum(cache_read_tokens) cr, sum(cache_write_tokens) cw
FROM sessions WHERE started_at > unixepoch('now', ?)
GROUP BY d ORDER BY d"""

TOP = """
SELECT id, source, api_call_count, cache_read_tokens, cache_write_tokens
FROM sessions WHERE started_at > unixepoch('now', ?)
ORDER BY cache_write_tokens DESC LIMIT 8"""


def per_call(tok, calls):
    return tok // calls if calls else 0


def summarize(rows):
    """rows = [(d, n, calls, out, in, cr, cw)] -> totals, ratios, weighted cost."""
    calls = sum(r[2] or 0 for r in rows)
    tok = {"output": sum(r[3] or 0 for r in rows), "input": sum(r[4] or 0 for r in rows),
           "cache_read": sum(r[5] or 0 for r in rows), "cache_write": sum(r[6] or 0 for r in rows)}
    usd = {k: tok[k] * PRICE[k] / 1e6 for k in tok}
    total = sum(usd.values())
    cr, cw = tok["cache_read"], tok["cache_write"]
    return {
        "sessions": sum(r[1] for r in rows), "calls": calls, **tok,
        "read_per_call": per_call(cr, calls), "write_per_call": per_call(cw, calls),
        "hit_rate": cr / (cr + cw) if cr + cw else 0.0,
        "read_over_output": cr / tok["output"] if tok["output"] else 0.0,
        "usd": usd, "usd_share": {k: v / total if total else 0.0 for k, v in usd.items()},
    }


def fmt(s):
    sh = s["usd_share"]
    return (f"sessions={s['sessions']} calls={s['calls']:,} read/call={s['read_per_call']:,} "
            f"write/call={s['write_per_call']:,} hit={s['hit_rate']:.1%} "
            f"read/output={s['read_over_output']:.0f}x | weighted $: write {sh['cache_write']:.0%} "
            f"read {sh['cache_read']:.0%} output {sh['output']:.0%}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--db", default=os.path.expanduser("~/.hermes/state.db"))
    ap.add_argument("--days", type=int, default=14)
    ap.add_argument("--split", help="split date YYYY-MM-DD (included in 'after')")
    ap.add_argument("--json", help="also write the daily data to this file")
    a = ap.parse_args()

    con = sqlite3.connect(f"file:{a.db}?mode=ro", uri=True)
    win = f"-{a.days} days"
    rows = con.execute(DAILY, (win,)).fetchall()
    if not rows:
        print("no session in the window")
        return 1

    print(f"{'day':10} {'sess':>4} {'calls':>6} {'output':>9} {'cache_read':>13} "
          f"{'read/call':>10} {'write/call':>11}")
    for d, n, calls, out, _, cr, cw in rows:
        print(f"{d:10} {n:>4} {calls or 0:>6} {out or 0:>9,} {cr or 0:>13,} "
              f"{per_call(cr or 0, calls):>10,} {per_call(cw or 0, calls):>11,}")

    tot = summarize(rows)
    print("\ntotal:", fmt(tot))
    print("weighted total $ (Opus 5.5 API prices): " +
          " ".join(f"{k}={v:,.0f}$" for k, v in tot["usd"].items()))
    if a.split:
        before = [r for r in rows if r[0] < a.split]
        after = [r for r in rows if r[0] >= a.split]
        if before and after:
            b, f = summarize(before), summarize(after)
            print(f"before {a.split}:", fmt(b))
            print(f"after {a.split}: ", fmt(f))
            for k in ("read_per_call", "write_per_call"):
                if b[k]:
                    print(f"{k}: {f[k] / b[k] - 1:+.0%}")

    if a.json:
        with open(a.json, "w") as f:
            json.dump({"split": a.split, "price_usd_per_mtok": PRICE, "days": [
                {"day": d, "sessions": n, "calls": c or 0, "output": o or 0, "input": i or 0,
                 "cache_read": cr or 0, "cache_write": cw or 0}
                for d, n, c, o, i, cr, cw in rows]}, f, indent=1)

    print("\nsessions writing the most to cache:")
    for sid, src, calls, cr, cw in con.execute(TOP, (win,)):
        print(f"  {sid[:28]:28} {src or '':8} calls={calls or 0:>5} cache_write={cw or 0:>11,} "
              f"write/call={per_call(cw or 0, calls):>7,} read/call={per_call(cr or 0, calls):>8,}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
