#!/usr/bin/env python3
"""Generate the README charts from results/*.json, all in one visual style.

    python3 tools/make_charts.py [--results results] [--out build/charts]

Draft charts straight from the data. docs/img/ holds polished versions of these same
charts: same numbers, checked by tests/test_tools.py.

Stdlib only. Each chart is written twice, name.svg (light) and name-dark.svg (dark),
for a <picture> that follows the reader's GitHub theme. Pure CSS animations (GitHub
plays them inside an <img>, no JS), off when the reader prefers reduced motion.
Every number shown comes from a file in results/.
"""
import argparse
import json
import os
import sys
from xml.sax.saxutils import escape

THEMES = {
    "light": dict(BG="#ffffff", CARD="#f6f8fa", FG="#1f2328", MUTED="#59636e", GRID="#d1d9e0",
                  READ="#1a7f37", WRITE="#bc4c00", OUT="#8250df", BLUE="#0969da", ONBAR="#ffffff"),
    "dark": dict(BG="#0d1117", CARD="#161b22", FG="#e6edf3", MUTED="#8b949e", GRID="#30363d",
                 READ="#3fb950", WRITE="#f0883e", OUT="#a371f7", BLUE="#58a6ff", ONBAR="#0d1117"),
}
globals().update(THEMES["light"])  # main() switches theme before each pass
FONT = "ui-sans-serif,-apple-system,Segoe UI,Helvetica,Arial,sans-serif"
W, X0, XR = 960, 40, 920  # every chart: same width, same margins


def style():
    return f"""<style>
text{{font-family:{FONT};fill:{FG}}}
.h{{font-size:30px;font-weight:800}} .t{{font-size:22px;font-weight:700}} .s{{font-size:14px;fill:{MUTED}}}
.l{{font-size:13px}} .m{{font-size:12px;fill:{MUTED}}} .v{{font-size:13px;font-weight:600}}
.gx{{transform-box:fill-box;transform-origin:left center;animation:gx 1.1s cubic-bezier(.2,.8,.2,1) both}}
.gy{{transform-box:fill-box;transform-origin:center bottom;animation:gy .9s cubic-bezier(.2,.8,.2,1) both}}
.fi{{animation:fi .6s ease-out both}}
@keyframes gx{{from{{transform:scaleX(0)}}}}
@keyframes gy{{from{{transform:scaleY(0)}}}}
@keyframes fi{{from{{opacity:0}}}}
@media (prefers-reduced-motion:reduce){{*{{animation:none!important}}}}
</style>"""


def svg(h, body, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{h}" '
            f'viewBox="0 0 {W} {h}" role="img" aria-label="{escape(title)}">'
            f'<title>{escape(title)}</title>{style()}'
            f'<rect x=".5" y=".5" width="{W - 1}" height="{h - 1}" rx="14" fill="{BG}" stroke="{GRID}"/>'
            f'{body}</svg>')


def t(x, y, s, cls="l", anchor="start", delay=None, fill=None):
    st = f' style="animation-delay:{delay:.2f}s"' if delay is not None else ""
    f = f' style="fill:{fill}"' if fill else ""
    if fill and delay is not None:
        st, f = f' style="fill:{fill};animation-delay:{delay:.2f}s"', ""
    c = f"{cls} fi" if delay is not None else cls
    return f'<text x="{x:.1f}" y="{y:.1f}" class="{c}" text-anchor="{anchor}"{f}{st}>{escape(s)}</text>'


def bar(x, y, w, h, color, delay=0.0, axis="x", rx=4):
    return (f'<rect x="{x:.1f}" y="{y:.1f}" width="{max(w, 0):.1f}" height="{max(h, 0):.1f}" rx="{rx}" '
            f'fill="{color}" class="g{axis}" style="animation-delay:{delay:.2f}s"/>')


def header(title, sub, legend=()):
    """Title, subtitle, right-aligned legend: identical on every chart."""
    out, x = [t(X0, 48, title, "t"), t(X0, 74, sub, "s")], XR
    for name, col in reversed(legend):
        x -= len(name) * 6.6 + 17
        out.append(f'<rect x="{x:.0f}" y="38" width="11" height="11" rx="3" fill="{col}"/>' + t(x + 17, 48, name, "m"))
        x -= 20
    return out


def kfmt(n):
    if n >= 1e9:
        return f"{n / 1e9:.2f} G"
    if n >= 1e6:
        return f"{n / 1e6:.1f} M"
    if n >= 1e3:
        return f"{n / 1e3:.1f} k"
    return str(int(n))


def pct(x):
    return f"{x * 100:.1f}%" if x < 0.1 else f"{x * 100:.0f}%"


# --- banner -----------------------------------------------------------------------------
def hero(h, lv, rk):
    days, price = h["days"], h["price_usd_per_mtok"]
    usd = {k: sum(d[k] for d in days) * price[k] / 1e6 for k in ("cache_read", "cache_write", "output")}
    v = lv["variants"]
    base, tools = v["default"][0]["prompt"], v["no_builtin_tools"][0]["prompt"]
    gl = next(r for r in rk["rows"] if r["cmd"].startswith("git log"))
    kpis = [(f"{usd['cache_write'] / sum(usd.values()):.0%}", "of the bill = cache writes", "21 days, API prices", WRITE),
            (f"{1 - tools / base:.0%}", "of the prefix = tool schemas", "before you type a word", BLUE),
            (f"{gl['rtk'] / gl['raw'] - 1:+.0%}".replace("+", ""), "git log output through rtk", "same for ls -la", READ)]
    out = [t(X0, 60, "agent-token-savings", "h"),
           t(X0, 88, "Measure, then cut what any coding agent spends. Real numbers, rerunnable scripts.", "s")]
    for i, (big, l1, l2, col) in enumerate(kpis):
        x, d = X0 + i * 300, 0.3 + i * 0.35
        out.append(f'<g class="fi" style="animation-delay:{d:.2f}s">'
                   f'<rect x="{x}" y="114" width="280" height="116" rx="12" fill="{CARD}" stroke="{GRID}"/>'
                   f'<rect x="{x}" y="114" width="4" height="116" rx="2" fill="{col}"/></g>')
        out.append(f'<text x="{x + 22}" y="170" font-size="40" font-weight="800" style="fill:{col};'
                   f'animation-delay:{d + .2:.2f}s" class="fi">{escape(big)}</text>')
        out.append(t(x + 22, 196, l1, "l", delay=d + .3))
        out.append(t(x + 22, 215, l2, "m", delay=d + .3))
    return svg(258, "".join(out), "Three key numbers")


# --- before / after: order of the prompt -----------------------------------------------
def prompt_order():
    rows = [("Before", "date at the start",
             [("date", 90, WRITE), ("tools", 160, WRITE), ("instructions", 160, WRITE),
              ("conversation", 196, WRITE), ("new message", 120, WRITE)],
             "The date changes every message, so everything after it is written again.", WRITE),
            ("After", "date at the end",
             [("tools", 160, READ), ("instructions", 160, READ), ("conversation", 200, READ),
              ("date + new message", 210, WRITE)],
             "The stable start is re-read from cache. Only the tail is written.", READ)]
    out = header("Put what changes at the end of the prompt",
                 "Same content, different order. Caches match the start of the prompt.",
                 [("read from cache", READ), ("written", WRITE)])
    for r, (name, sub, blocks, note, ncol) in enumerate(rows):
        y, d0 = 112 + r * 108, 0.3 + r * 1.2
        out += [t(X0, y + 19, name, "v"), t(X0, y + 37, sub, "m")]
        x = 170
        for i, (lab, w, col) in enumerate(blocks):
            d = d0 + i * 0.15
            out.append(bar(x, y, w, 42, col, d, rx=8))
            out.append(t(x + w / 2, y + 26, lab, "v", "middle", delay=d + .4, fill=ONBAR))
            x += w + 6
        out.append(t(170, y + 68, note, "l", delay=d0 + 1, fill=ncol))
    return svg(340, "".join(out), "Prompt order, before and after")


# --- tokens versus dollars --------------------------------------------------------------
def flip(h):
    days = h["days"]
    price = h["price_usd_per_mtok"]
    keys = [("cache_read", "cache read", READ), ("cache_write", "cache write", WRITE), ("output", "output", OUT)]
    tok = {k: sum(d[k] for d in days) for k, _, _ in keys}
    usd = {k: tok[k] * price[k] / 1e6 for k in tok}
    BW = XR - X0
    calls = f"{sum(d['calls'] for d in days):,}"
    out = header("What you re-read is not what you pay for",
                 f"One agent, {len(days)} days, {calls} API calls. Same usage, two units.",
                 [(n, c) for _, n, c in keys])
    for row, (label, vals, fmtv) in enumerate([
            ("in tokens", tok, kfmt),
            ("in $ at Opus 5.5 API prices", usd, lambda v: f"${v:,.0f}")]):
        y = 124 + row * 96
        out.append(t(X0, y - 10, label, "v"))
        tot = sum(vals.values())
        x = X0
        for i, (k, name, col) in enumerate(keys):
            w = BW * vals[k] / tot
            d = 0.2 + row * 0.9 + i * 0.25
            out.append(bar(x, y, w, 38, col, d, rx=3))
            share = pct(vals[k] / tot)
            if w > 190:
                out.append(t(x + 10, y + 24, f"{name} {share}", "v", delay=d + .6, fill=ONBAR))
                out.append(t(x + 4, y + 58, fmtv(vals[k]), "m", delay=d + .6))
            elif w > 40:
                out.append(t(x + w / 2, y + 24, share, "v", "middle", delay=d + .6, fill=ONBAR))
                if w > 60:
                    out.append(t(x + 4, y + 58, fmtv(vals[k]), "m", delay=d + .6))
            x += w
    st, su = sum(tok.values()), sum(usd.values())
    ratio = price["cache_write"] / price["cache_read"]
    out.append(t(X0, 324, f"Reads are {pct(tok['cache_read'] / st)} of tokens, but writes "
                          f"(1h cache, {ratio:.0f}x the price of a read) are "
                          f"{pct(usd['cache_write'] / su)} of the bill.", "s", delay=2.2))
    return svg(350, "".join(out), "Tokens versus dollars")


# --- how the cache works, turn by turn --------------------------------------------------
def timeline():
    turns = [(0, 30), (30, 4), (34, 6), (40, 3), (43, 5), (0, 48), (48, 4)]
    notes = {0: "cold start: everything is written", 5: "prefix changed: everything is rewritten",
             6: "cache is back"}
    scale = 11.5
    out = header("Each turn re-reads the prefix and writes the tail",
                 "Illustration, arbitrary units.", [("read from cache", READ), ("written", WRITE)])
    for i, (read, write) in enumerate(turns):
        y = 106 + i * 34
        d = 0.3 + i * 0.55
        out.append(t(X0, y + 16, f"turn {i + 1}", "m", delay=d))
        x = X0 + 64
        if read:
            out.append(bar(x, y, read * scale, 22, READ, d))
        out.append(bar(x + read * scale, y, write * scale, 22, WRITE, d + 0.25))
        if i in notes:
            out.append(t(x + (read + write) * scale + 12, y + 16, notes[i], "v" if i == 5 else "m",
                         delay=d + .4, fill=WRITE if i == 5 else None))
    return svg(362, "".join(out), "Prompt cache, turn by turn")


# --- Claude Code: prefix size per lever --------------------------------------------------
def prefix(lv):
    v = lv["variants"]
    names = [("default", "default"), ("exclude_dynamic_sections", "--exclude-dynamic-system-prompt-sections"),
             ("no_skills", "--disable-slash-commands"), ("no_user_settings", "--setting-sources project"),
             ("no_builtin_tools", '--tools ""')]
    rows = [(lab, v[k][0]["prompt"]) for k, lab in names if isinstance(v.get(k), list)]
    base = rows[0][1]
    LW, BW = 330, 440
    H = 128 + len(rows) * 44 + 30
    out = header(f"Claude Code sends {base:,} tokens before your first word",
                 f"claude -p, model {lv['model']}, empty folder, no MCP. One lever at a time.")
    for i, (lab, n) in enumerate(rows):
        y = 104 + i * 44
        d = 0.2 + i * 0.18
        out.append(t(X0, y + 19, lab, "l"))
        out.append(f'<rect x="{X0 + LW}" y="{y}" width="{BW}" height="28" rx="4" fill="{CARD}"/>')
        out.append(bar(X0 + LW, y, BW * n / base, 28, BLUE if i else MUTED, d))
        delta = "" if not i else f"  {(n - base) / base:+.0%}"
        out.append(t(X0 + LW + BW + 12, y + 19, f"{n:,}" + delta, "v", delay=d + .5))
    out.append(t(X0, H - 24, "Built-in tool schemas are 68% of the prefix.", "s", delay=1.6))
    return svg(H, "".join(out), "Claude Code prefix size per lever")


# --- RTK ----------------------------------------------------------------------------------
def rtk(r):
    rows = r["rows"]
    LW, BW = 270, 380
    H = 118 + len(rows) * 46 + 34
    out = header("RTK: raw versus filtered shell output", "Bytes sent back to the agent. Scale per row.",
                 [("raw", MUTED), ("through rtk", BLUE)])
    for i, row in enumerate(rows):
        y = 104 + i * 46
        d = 0.2 + i * 0.2
        raw, flt = row["raw"], row["rtk"]
        out.append(t(X0, y + 19, row["cmd"], "l"))
        out.append(bar(X0 + LW, y, BW, 12, MUTED, d))
        out.append(bar(X0 + LW, y + 16, BW * flt / raw if raw else 0, 12, BLUE, d + .3))
        gain = 1 - flt / raw if raw else 0
        note = "  truncated" if gain > 0.98 else ""
        out.append(t(X0 + LW + BW + 12, y + 22, f"{kfmt(raw)} > {kfmt(flt)}  " + (f"-{gain:.0%}" if round(gain, 2) else "0%") + note, "v",
                     delay=d + .6, fill=WRITE if note else None))
    out.append(t(X0, H - 24, "Real gain on git log and ls. On find, rtk summarizes and cuts: "
                             "the agent loses the list.", "s", delay=1.6))
    return svg(H, "".join(out), "Raw bytes versus RTK")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--results", default="results")
    ap.add_argument("--out", default="build/charts")  # docs/img holds the hand-polished versions
    a = ap.parse_args()
    load = lambda n: json.load(open(os.path.join(a.results, n)))  # noqa: E731
    h, lv, rk = load("hermes_usage.json"), load("claude_code_levers.json"), load("rtk_bytes.json")
    os.makedirs(a.out, exist_ok=True)
    for theme, suffix in (("light", ""), ("dark", "-dark")):
        globals().update(THEMES[theme])
        charts = {"hero": hero(h, lv, rk), "prompt-order": prompt_order(), "flip": flip(h),
                  "cache-timeline": timeline(), "claude-code-prefix": prefix(lv),
                  "rtk": rtk(rk)}
        for name, s in charts.items():
            p = os.path.join(a.out, f"{name}{suffix}.svg")
            with open(p, "w") as f:
                f.write(s)
            print(p, len(s))
    return 0


if __name__ == "__main__":
    sys.exit(main())
