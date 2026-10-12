#!/usr/bin/env python3
"""Real RTK bench: bytes of raw output vs `rtk <cmd>` on common commands.

    python3 bench/rtk_bytes.py [--repo PATH]

Measures bytes, not billed tokens: RTK only touches shell output
(not read_file / search_files, not the prefix, not the model output).
"""
import argparse
import json
import os
import shutil
import subprocess
import sys

CMDS = [
    "git status",
    "git log -n 30",
    "git diff HEAD~3 --stat",
    "ls -la",
    "find . -name '*.py' -maxdepth 3",
]


def size(cmd, cwd):
    p = subprocess.run(cmd, shell=True, capture_output=True, text=True, cwd=cwd, timeout=60)
    return len((p.stdout + p.stderr).encode())


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default=".", help="git repo to measure on (default: current directory)")
    ap.add_argument("--json", help="also write the measurements to this file")
    a = ap.parse_args()
    if not shutil.which("rtk"):
        print("rtk not found in PATH")
        return 1
    tr = tw = 0
    rows = []
    print(f"{'command':34} {'raw':>8} {'rtk':>8} {'gain':>6}")
    for c in CMDS:
        raw, rtk = size(c, a.repo), size(f"rtk {c}", a.repo)
        tr, tw = tr + raw, tw + rtk
        rows.append({"cmd": c, "raw": raw, "rtk": rtk})
        g = 1 - rtk / raw if raw else 0
        print(f"{c:34} {raw:>8,} {rtk:>8,} {g:>6.0%}")
    print(f"{'TOTAL':34} {tr:>8,} {tw:>8,} {1 - tw / tr:>6.0%}")
    if a.json:
        with open(a.json, "w") as f:
            json.dump({"repo": a.repo, "rows": rows}, f, indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
