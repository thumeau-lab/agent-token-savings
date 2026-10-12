#!/usr/bin/env python3
"""Find what breaks the cache: capture an agent's real requests, then compare them.

    python3 tools/cache_diff.py proxy DIR [--port 8977]    # then, elsewhere:
    ANTHROPIC_BASE_URL=http://127.0.0.1:8977 claude -p "ok"  # twice
    python3 tools/cache_diff.py diff DIR/000.json DIR/001.json

The proxy forwards to api.anthropic.com and records ONLY the JSON body of
/v1/messages requests: headers (token, API key) never touch the disk.
The diff splits each request in cache order (tools, system, messages) and
names the first block that differs: everything after it is rewritten to cache.
Stdlib only.
"""
import argparse
import difflib
import http.server
import itertools
import json
import os
import sys
import urllib.error
import urllib.request

UPSTREAM = "https://api.anthropic.com"
HOP = {"host", "content-length", "accept-encoding", "transfer-encoding", "connection", "content-encoding"}


def blocks(req):
    """Ordered list of (label, text), in the order the cache reads the prefix."""
    out = [(f"tools[{i}] {t.get('name')}", json.dumps(t, sort_keys=True)) for i, t in enumerate(req.get("tools", []))]
    system = req.get("system", [])
    for i, s in enumerate([{"text": system}] if isinstance(system, str) else system):
        out.append((f"system[{i}]", s.get("text", "")))
    for i, m in enumerate(req.get("messages", [])):
        content = [{"text": m["content"]}] if isinstance(m["content"], str) else m["content"]
        for j, c in enumerate(content):
            out.append((f"messages[{i}].{j} {m['role']}", c.get("text") or json.dumps(c, sort_keys=True)))
    return out


def first_diff(a, b):
    """(index, label) of the first differing block, or None if b extends a."""
    for i, ((la, ta), (_, tb)) in enumerate(zip(a, b)):
        if ta != tb:
            return i, la
    return None


def diff(fa, fb, context=2):
    a, b = (blocks(json.load(open(f))) for f in (fa, fb))
    hit = first_diff(a, b)
    if hit is None:
        print(f"identical prefix over {min(len(a), len(b))} blocks: nothing to rewrite")
        return 0
    i, label = hit
    rest = sum(len(t) for _, t in b[i:])
    print(f"first differing block: {label} (block {i + 1}/{len(b)}), ~{rest:,} characters rewritten from here")
    texts_a = sorted(t for _, t in a[i:])
    if texts_a == sorted(t for _, t in b[i:]):
        print("same blocks, different order: sources injected in a non-deterministic order (parallel hooks?)")
    for line in difflib.unified_diff(a[i][1].splitlines(), b[i][1].splitlines(), lineterm="", n=context):
        print(" ", line[:160])
    return 1


class Proxy(http.server.BaseHTTPRequestHandler):
    out, seq = ".", itertools.count()

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        if self.path.startswith("/v1/messages") and "count_tokens" not in self.path:
            with open(os.path.join(self.out, f"{next(self.seq):03d}.json"), "wb") as f:
                f.write(body)
        hdr = {k: v for k, v in self.headers.items() if k.lower() not in HOP}
        try:
            r = urllib.request.urlopen(urllib.request.Request(UPSTREAM + self.path, body, hdr), timeout=300)
        except urllib.error.HTTPError as e:
            r = e
        data = r.read()  # note: buffered response, streaming arrives in one chunk; enough for claude -p
        self.send_response(r.status)
        for k, v in r.headers.items():
            if k.lower() not in HOP:
                self.send_header(k, v)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *a):
        pass


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("proxy")
    p.add_argument("dir")
    p.add_argument("--port", type=int, default=8977)
    d = sub.add_parser("diff")
    d.add_argument("a")
    d.add_argument("b")
    a = ap.parse_args()
    if a.cmd == "diff":
        return diff(a.a, a.b)
    os.makedirs(a.dir, exist_ok=True)
    Proxy.out = a.dir
    print(f"proxy on 127.0.0.1:{a.port}, bodies in {a.dir}", flush=True)
    http.server.ThreadingHTTPServer(("127.0.0.1", a.port), Proxy).serve_forever()


if __name__ == "__main__":
    sys.exit(main())
