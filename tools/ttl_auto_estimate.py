"""Estimate the switch from cache_ttl 1h to auto on machine sessions (subagent, cron...).

For each machine session: cache writes, and the largest gap between two messages.
Gap > 5 min = a 5m TTL would have lost the cache and rewritten the prefix (extra cost).
Prices relative to base input: 1h write = 2x, 5m write = 1.25x.

    python3 tools/ttl_auto_estimate.py ~/.hermes
"""
import glob, sqlite3, sys, time

MACHINE = {"subagent", "cron", "oneshot", "webhook", "kanban", "api", "tool", "batch"}
home = sys.argv[1]
since = time.time() - 30 * 86400  # last 30 days
tot = {"w": 0, "sess": 0, "gap": 0, "w_gap": 0, "w_all": 0}
for db in [f"{home}/state.db"] + sorted(glob.glob(f"{home}/profiles/*/state.db")):
    c = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    for sid, src, w in c.execute(
            "select id, source, coalesce(cache_write_tokens,0) from sessions where started_at>?", (since,)):
        tot["w_all"] += w
        if (src or "") not in MACHINE or not w:
            continue
        ts = [r[0] for r in c.execute("select timestamp from messages where session_id=? order by timestamp", (sid,))]
        gap = max((b - a for a, b in zip(ts, ts[1:])), default=0)
        tot["sess"] += 1
        tot["w"] += w
        if gap > 300:
            tot["gap"] += 1
            tot["w_gap"] += w
w, wa = tot["w"], tot["w_all"] or 1
print(f"machine sessions={tot['sess']}  of which gap>5min={tot['gap']}")
print(f"machine writes={w/1e6:.1f}M of {wa/1e6:.1f}M total ({w/wa:.0%})")
print(f"sessions without gap>5min: writes {(w-tot['w_gap'])/1e6:.1f}M, saving on these with 5m = 37.5%")
print(f"estimated saving on total writes: {(w-tot['w_gap'])*0.375/wa:.1%} (lower bound, sessions with gaps not counted)")
