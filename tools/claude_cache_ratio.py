#!/usr/bin/env python3
"""Measure Claude Code prompt caching from the local transcripts.

No install, no proxy: the JSONL files in ~/.claude/projects/ already carry
`cache_creation_input_tokens` / `cache_read_input_tokens` per call.

    python3 tools/claude_cache_ratio.py [root]   # default ~/.claude

Three sections:
  1. Totals + hit rate  -> is the cache read or rebuilt?
  2. Shape per session  -> where the cache writes fall in the sequence.
  3. 'resume' signature -> massive write while the context is ALREADY warm.

(3) is what decides whether a cache fix is worth adopting. (1) alone is
misleading: a non-zero write ratio is normal, the context grows every turn
and the tail of the prefix gets cached again.
"""
import glob
import json
import os
import sys

WARN_CREATE = 10_000   # "big write" threshold for the resume signature
BIG_CREATE = 2_000     # threshold for showing 'C' in the shape


def parse(path):
    """Usage records of one session, deduplicated.

    Some agents re-emit the same `usage` block on every event of a streamed turn;
    without dedup the totals get multiplied by the number of events.
    """
    seen, rows = set(), []
    for line in open(path, errors="replace"):
        line = line.strip()
        if not line:
            continue
        try:
            d = json.loads(line)
        except ValueError:
            continue
        u = (d.get("message") or {}).get("usage")
        if not isinstance(u, dict):
            continue
        key = (u.get("input_tokens"), u.get("output_tokens"),
               u.get("cache_read_input_tokens"), u.get("cache_creation_input_tokens"))
        if key in seen:
            continue
        seen.add(key)
        rows.append({
            "ts": d.get("timestamp"),
            "inp": u.get("input_tokens") or 0,
            "out": u.get("output_tokens") or 0,
            "read": u.get("cache_read_input_tokens") or 0,
            "create": u.get("cache_creation_input_tokens") or 0,
        })
    return rows


def main(root=None):
    root = os.path.expanduser(root or "~/.claude")
    paths = sorted(glob.glob(os.path.join(root, "projects", "*", "*.jsonl")),
                   key=os.path.getmtime)
    tot = dict(c=0, r=0, i=0, o=0)
    per, hits, first_cold, n = [], [], 0, 0

    for p in paths:
        rows = parse(p)
        if not rows:
            continue
        n += 1
        c = sum(x["create"] for x in rows)
        r = sum(x["read"] for x in rows)
        tot["c"] += c
        tot["r"] += r
        tot["i"] += sum(x["inp"] for x in rows)
        tot["o"] += sum(x["out"] for x in rows)
        per.append((c, r, rows, os.path.basename(p)))
        if rows[0]["read"] == 0:
            first_cold += 1
        for idx, x in enumerate(rows[1:], 1):
            warm = any(y["read"] > 0 for y in rows[:idx])
            if warm and x["read"] == 0 and x["create"] > WARN_CREATE:
                hits.append((os.path.basename(p), idx, x["create"]))

    if not n:
        print(f"no usable transcript under {root}/projects/")
        return 1

    denom = tot["c"] + tot["r"]
    print(f"sessions with usage  : {n}")
    print(f"cache_creation total : {tot['c']:>12,}")
    print(f"cache_read total     : {tot['r']:>12,}")
    print(f"uncached input       : {tot['i']:>12,}")
    print(f"output               : {tot['o']:>12,}")
    if denom:
        print(f"hit rate cache       : {tot['r'] / denom:>11.1%}")

    print("\n-- shape of the 10 sessions writing the most --")
    print("   C=write >2k   .=small write   _=read only")
    for c, r, rows, name in sorted(per, key=lambda t: -t[0])[:10]:
        seq = "".join("C" if x["create"] > BIG_CREATE else ("." if x["create"] else "_")
                      for x in rows)
        print(f"{name[:26]:26} n={len(rows):3} create={c:>9,} read={r:>10,} {seq[:60]}")

    print(f"\n-- 'resume' signature (warm context, then create>{WARN_CREATE:,} with read=0) --")
    print(f"cold first call (normal)   : {first_cold}/{n}")
    print(f"warm rebuilds (abnormal)   : {len(hits)}/{n}")
    for name, idx, c in hits[:20]:
        print(f"  {name[:30]:30} msg#{idx:<3} create={c:,}")
    if not hits:
        print("  none -> the unstable-prefix bug does not show here,")
        print("          a fixing proxy would have nothing to fix.")
    return 0


def _selftest():
    import tempfile
    lines = [
        {"message": {"usage": {"input_tokens": 1, "output_tokens": 5,
                               "cache_read_input_tokens": 0, "cache_creation_input_tokens": 30000}}},
        # streaming duplicate: must be ignored
        {"message": {"usage": {"input_tokens": 1, "output_tokens": 5,
                               "cache_read_input_tokens": 0, "cache_creation_input_tokens": 30000}}},
        {"message": {"usage": {"input_tokens": 1, "output_tokens": 5,
                               "cache_read_input_tokens": 30000, "cache_creation_input_tokens": 900}}},
        # anomaly: warm, then massive rebuild without a read
        {"message": {"usage": {"input_tokens": 2, "output_tokens": 6,
                               "cache_read_input_tokens": 0, "cache_creation_input_tokens": 50000}}},
    ]
    with tempfile.TemporaryDirectory() as d:
        p = os.path.join(d, "s.jsonl")
        with open(p, "w") as f:
            for l in lines:
                f.write(json.dumps(l) + "\n")
        rows = parse(p)
        assert len(rows) == 3, rows              # dedup
        assert sum(r["create"] for r in rows) == 80900
        warm_anomalies = [i for i, x in enumerate(rows)
                          if i and any(y["read"] > 0 for y in rows[:i])
                          and x["read"] == 0 and x["create"] > WARN_CREATE]
        assert warm_anomalies == [2], warm_anomalies
    print("selftest ok")


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        _selftest()
    else:
        sys.exit(main(*[a for a in sys.argv[1:] if not a.startswith("-")]))
