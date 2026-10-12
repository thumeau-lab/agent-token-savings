#!/usr/bin/env python3
"""Real bench: size of the Claude Code prefix for each CLI lever.

Each variant runs `claude -p` twice (same prompt, same cwd) and records
`usage`. Call 1 = cold cost of the prefix, call 2 = is the cache re-read?

    python3 bench/claude_code_levers.py [--model haiku] [--out results/x.json]

Uses real quota (~2 short calls per variant). Variants are independent:
each changes only ONE lever compared to `default`.
"""
import argparse
import json
import os
import subprocess
import sys
import tempfile

PROMPT = "Reply with the single word: ok"

# name -> (extra CLI args, extra env)
VARIANTS = {
    "default": ([], {}),
    "no_prompt_caching": ([], {"DISABLE_PROMPT_CACHING": "1"}),
    "no_builtin_tools": (["--tools", ""], {}),
    "no_skills": (["--disable-slash-commands"], {}),
    "strict_mcp_none": (["--strict-mcp-config"], {}),
    "no_user_settings": (["--setting-sources", "project"], {}),
    "exclude_dynamic_sections": (["--exclude-dynamic-system-prompt-sections"], {}),
}


def prompt_tokens(u):
    """Total prompt size seen by the model, whatever the cache."""
    return (u.get("input_tokens", 0) + u.get("cache_creation_input_tokens", 0)
            + u.get("cache_read_input_tokens", 0))


def run(model, extra_args, extra_env, cwd):
    cmd = ["claude", "-p", PROMPT, "--model", model, "--output-format", "json",
           "--no-session-persistence", *extra_args]
    env = {**os.environ, **extra_env}
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=180, env=env, cwd=cwd)
    d = json.loads(p.stdout)
    u = d.get("usage") or {}
    return {
        "is_error": d.get("is_error"),
        "input": u.get("input_tokens", 0),
        "create": u.get("cache_creation_input_tokens", 0),
        "read": u.get("cache_read_input_tokens", 0),
        "output": u.get("output_tokens", 0),
        "prompt": prompt_tokens(u),
        "ttl_1h": (u.get("cache_creation") or {}).get("ephemeral_1h_input_tokens", 0),
        "cost_usd": d.get("total_cost_usd"),
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", default="haiku")
    ap.add_argument("--out", default="results/claude_code_levers.json")
    ap.add_argument("--only", nargs="*", help="subset of variants")
    a = ap.parse_args()

    out = {}
    with tempfile.TemporaryDirectory() as cwd:  # neutral cwd: no project CLAUDE.md
        for name, (args, env) in VARIANTS.items():
            if a.only and name not in a.only:
                continue
            try:
                calls = [run(a.model, args, env, cwd) for _ in range(2)]
            except Exception as e:  # one broken variant does not kill the bench
                out[name] = {"error": repr(e)}
                print(f"{name:26} ERROR {e!r}", file=sys.stderr)
                continue
            out[name] = calls
            c1, c2 = calls
            print(f"{name:26} prompt={c1['prompt']:>7,}  call1 create={c1['create']:>7,} "
                  f"read={c1['read']:>7,} | call2 create={c2['create']:>7,} read={c2['read']:>7,}")

    base = (out.get("default") or [{}])[0].get("prompt")
    if base:
        print("\nprefix delta vs default (call 1):")
        for name, calls in out.items():
            if isinstance(calls, list):
                d = calls[0]["prompt"] - base
                print(f"  {name:26} {d:+8,} tok ({d / base:+.1%})")

    os.makedirs(os.path.dirname(a.out) or ".", exist_ok=True)
    with open(a.out, "w") as f:
        json.dump({"model": a.model, "prompt": PROMPT, "variants": out}, f, indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
