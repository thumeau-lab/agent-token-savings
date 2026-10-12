"""Tests hors reseau des outils de mesure : python3 -m pytest -q"""
import json
import sqlite3
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path[:0] = [str(ROOT / "tools"), str(ROOT / "bench")]

import claude_cache_ratio as ccr  # noqa: E402
import claude_code_levers as ccl  # noqa: E402
import hermes_usage as hu  # noqa: E402


def _usage(inp, out, read, create):
    return {"message": {"usage": {"input_tokens": inp, "output_tokens": out,
                                  "cache_read_input_tokens": read,
                                  "cache_creation_input_tokens": create}}}


def test_cache_ratio_dedups_streaming_events(tmp_path):
    p = tmp_path / "s.jsonl"
    lines = [_usage(1, 5, 0, 30000), _usage(1, 5, 0, 30000), _usage(1, 5, 30000, 900)]
    p.write_text("\n".join(json.dumps(x) for x in lines) + "\nnot json\n")
    rows = ccr.parse(str(p))
    assert len(rows) == 2
    assert sum(r["create"] for r in rows) == 30900


def test_cache_ratio_flags_warm_recreation(tmp_path, capsys):
    proj = tmp_path / "projects" / "x"
    proj.mkdir(parents=True)
    lines = [_usage(1, 5, 0, 30000), _usage(1, 5, 30000, 900), _usage(2, 6, 0, 50000)]
    (proj / "a.jsonl").write_text("\n".join(json.dumps(x) for x in lines))
    assert ccr.main(str(tmp_path)) == 0
    out = capsys.readouterr().out
    assert "warm rebuilds (abnormal)   : 1/1" in out


def test_levers_prompt_tokens_sums_all_three_terms():
    u = {"input_tokens": 10, "cache_creation_input_tokens": 200, "cache_read_input_tokens": 3000}
    assert ccl.prompt_tokens(u) == 3210
    assert ccl.prompt_tokens({}) == 0


def test_hermes_usage_summarize_ratios():
    rows = [("2026-10-01", 2, 10, 100, 0, 1000, 0), ("2026-10-02", 1, 0, 0, 0, 0, 0)]
    s = hu.summarize(rows)
    assert s["read_per_call"] == 100
    assert s["hit_rate"] == 1.0
    assert s["read_over_output"] == 10
    assert hu.per_call(5, 0) == 0  # pas de division par zero


def test_hermes_usage_weighted_cost_flips_the_ranking():
    # 1M lus, 100k ecrits, 10k sortis : la lecture domine en tokens, l'ecriture en dollars.
    s = hu.summarize([("2026-10-01", 1, 10, 10_000, 0, 1_000_000, 100_000)])
    assert s["read_over_output"] == 100
    assert s["usd"]["cache_write"] == 0.8 and s["usd"]["cache_read"] == 0.2
    assert max(s["usd_share"], key=s["usd_share"].get) == "cache_write"
    assert abs(sum(s["usd_share"].values()) - 1) < 1e-9
    assert s["write_per_call"] == 10_000


def test_hermes_usage_cli_on_fake_db(tmp_path):
    db = tmp_path / "state.db"
    con = sqlite3.connect(db)
    con.execute("CREATE TABLE sessions (id TEXT, source TEXT, started_at REAL, api_call_count INT,"
                " input_tokens INT, output_tokens INT, cache_read_tokens INT, cache_write_tokens INT)")
    now = time.time()
    con.executemany("INSERT INTO sessions VALUES (?,?,?,?,?,?,?,?)", [
        ("old", "cli", now - 5 * 86400, 10, 1, 50, 2_000_000, 10),
        ("new", "cli", now, 10, 1, 50, 500_000, 10),
    ])
    con.commit()
    con.close()
    split = time.strftime("%Y-%m-%d", time.localtime(now - 86400))
    p = subprocess.run([sys.executable, str(ROOT / "tools/hermes_usage.py"), "--db", str(db),
                        "--days", "7", "--split", split], capture_output=True, text=True)
    assert p.returncode == 0, p.stderr
    assert "read_per_call: -75%" in p.stdout


def test_make_charts_renders_valid_svgs_from_results(tmp_path):
    import xml.dom.minidom
    p = subprocess.run([sys.executable, str(ROOT / "tools" / "make_charts.py"),
                        "--results", str(ROOT / "results"), "--out", str(tmp_path)],
                       capture_output=True, text=True)
    assert p.returncode == 0, p.stderr
    files = sorted(f.name for f in tmp_path.glob("*.svg"))
    assert files == sorted(f"{n}{s}.svg" for n in ['cache-timeline', 'claude-code-prefix', 'flip', 'hero', 'prompt-order', 'rtk'] for s in ("", "-dark"))
    for f in tmp_path.glob("*.svg"):
        xml.dom.minidom.parse(str(f))  # XML valide, sinon GitHub affiche une image cassee
        assert "prefers-reduced-motion" in f.read_text()
    assert "-61%" in (tmp_path / "hero.svg").read_text()


def test_cache_diff_names_first_changed_block_and_spots_reordering():
    sys.path.insert(0, str(ROOT / "tools"))
    import cache_diff as cd

    def req(*first):
        return {"tools": [{"name": "Bash"}], "system": [{"text": "sys"}],
                "messages": [{"role": "user", "content": [{"text": t} for t in first]}]}

    a, b = cd.blocks(req("hookA", "hookB", "ok")), cd.blocks(req("hookB", "hookA", "ok"))
    assert cd.first_diff(a, a) is None
    assert cd.first_diff(a, b) == (2, "messages[0].0 user")
    assert cd.first_diff(cd.blocks(req("ok")), cd.blocks({**req("ok"), "tools": [{"name": "Read"}]})) == (0, "tools[0] Bash")


def test_polished_charts_keep_the_generated_numbers(tmp_path):
    """docs/img is hand-polished: it must still show every number make_charts derives from results/."""
    import re
    subprocess.run([sys.executable, str(ROOT / "tools" / "make_charts.py"), "--out", str(tmp_path)], check=True)
    num = re.compile(r"[-+]?\d[\d.,]*\s?[%kMG]?")
    for gen in tmp_path.glob("*.svg"):
        shown = (ROOT / "docs" / "img" / gen.name).read_text()
        texts = lambda s: " ".join(re.findall(r">([^<]+)</text>", s))  # noqa: E731
        missing = set(num.findall(texts(gen.read_text()))) - set(num.findall(texts(shown)))
        assert not missing, (gen.name, missing)
