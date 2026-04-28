#!/usr/bin/env python3
# Copyright 2026 The rpcduel Authors
# SPDX-License-Identifier: Apache-2.0
"""
review_trace_diffs.py — walk through a `rpcduel replay --csv` mismatch file
one row at a time, re-execute the JSON-RPC call against two endpoints, and
show the diverging sub-tree side-by-side. Press Enter to advance.

Usage:
    ./scripts/review_trace_diffs.py \\
        --csv diffs.csv \\
        --rpc-a https://node-a.example/ \\
        --rpc-b https://node-b.example/

Optional flags:
    --start N         skip the first N rows (default 0)
    --filter SUBSTR   only show rows whose `detail` contains SUBSTR
    --context N       number of sibling keys to show around the diff (default 6)
    --timeout SECS    HTTP timeout (default 30)
    --no-color        disable ANSI colors

The CSV is expected to have the columns:
    category,method,params,detail
matching `rpcduel replay --csv` output. Only rows whose category contains
`mismatch` are reviewed; others are skipped.

The script has zero third-party dependencies: stdlib only (urllib, csv, json,
shutil).
"""
from __future__ import annotations

import argparse
import csv
import json
import os
import re
import shutil
import sys
import urllib.error
import urllib.request
from typing import Any, Iterable

# ---------- ANSI helpers ------------------------------------------------------

USE_COLOR = sys.stdout.isatty()


def c(code: str, s: str) -> str:
    if not USE_COLOR:
        return s
    return f"\033[{code}m{s}\033[0m"


def red(s: str) -> str:    return c("31", s)
def green(s: str) -> str:  return c("32", s)
def yellow(s: str) -> str: return c("33;1", s)
def blue(s: str) -> str:   return c("34;1", s)
def dim(s: str) -> str:    return c("2", s)
def bold(s: str) -> str:   return c("1", s)


# ---------- JSONPath-ish navigation ------------------------------------------

PATH_TOKEN_RE = re.compile(r"\.([A-Za-z_][A-Za-z0-9_]*)|\[(\d+)\]")


def parse_path(path: str) -> list:
    """Parse "$[1].calls[1].gasUsed" -> [1, "calls", 1, "gasUsed"]."""
    if not path or not path.startswith("$"):
        return []
    parts: list = []
    for m in PATH_TOKEN_RE.finditer(path[1:]):
        key, idx = m.group(1), m.group(2)
        if key is not None:
            parts.append(key)
        else:
            parts.append(int(idx))
    return parts


def walk(doc: Any, parts: list) -> Any:
    cur = doc
    for p in parts:
        try:
            cur = cur[p]
        except (KeyError, IndexError, TypeError):
            return _MISSING
    return cur


class _Missing:
    def __repr__(self): return "<missing>"


_MISSING = _Missing()


# ---------- RPC ---------------------------------------------------------------

def rpc_call(url: str, method: str, params: list, timeout: float) -> Any:
    body = json.dumps({
        "jsonrpc": "2.0", "id": 1, "method": method, "params": params,
    }).encode()
    req = urllib.request.Request(
        url, data=body,
        headers={"content-type": "application/json"},
    )
    raw: bytes
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
    except urllib.error.HTTPError as e:
        # JSON-RPC servers often return non-2xx with an error body — read it.
        try:
            raw = e.read()
        except Exception:
            raw = b""
        if not raw:
            return {"__http_error__": f"HTTP {e.code} {e.reason}"}
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {"__http_error__": raw.decode("utf-8", "replace")[:2000]}
    if isinstance(payload, dict) and payload.get("error") is not None:
        return {"__rpc_error__": payload["error"]}
    if isinstance(payload, dict):
        return payload.get("result")
    return payload


# ---------- Side-by-side render ----------------------------------------------

def term_width() -> int:
    try:
        return max(60, shutil.get_terminal_size((120, 24)).columns)
    except OSError:
        return 120


def jdump(v: Any) -> str:
    if v is _MISSING:
        return "<missing>"
    return json.dumps(v, indent=2, sort_keys=True, ensure_ascii=False)


def side_by_side(left: str, right: str, header_l: str, header_r: str) -> None:
    width = term_width()
    col = (width - 3) // 2  # 3 = " | "
    sep = dim(" | ")

    def hpad(s: str, w: int) -> str:
        # truncate / pad ignoring ANSI (headers/lines passed in are plain)
        if len(s) >= w:
            return s[: w - 1] + "…"
        return s + " " * (w - len(s))

    # Header
    print(bold(blue(hpad(header_l, col))) + sep + bold(red(hpad(header_r, col))))
    print(dim("-" * col) + sep + dim("-" * col))

    ll = left.splitlines() or [""]
    rl = right.splitlines() or [""]
    n = max(len(ll), len(rl))
    for i in range(n):
        l = ll[i] if i < len(ll) else ""
        r = rl[i] if i < len(rl) else ""
        marker = "  " if l == r else dim("≠ ")
        # color full lines
        lcol = blue(hpad(l, col)) if l != r else hpad(l, col)
        rcol = red(hpad(r, col)) if l != r else hpad(r, col)
        print(marker + lcol + sep + rcol)


# ---------- Detail parsing ---------------------------------------------------

# `detail` looks like: "[$[1].calls[1].gasUsed] 0x39a5 vs 0x4a0d (value mismatch)"
DETAIL_RE = re.compile(
    r"^\[(?P<path>[^\]]+)\]\s+"
    r"(?P<left>\S+)\s+vs\s+(?P<right>\S+)\s+"
    r"\((?P<reason>[^)]+)\)\s*$"
)


def parse_detail(s: str) -> dict | None:
    m = DETAIL_RE.match(s.strip())
    if not m:
        return None
    return {
        "path": m.group("path"),
        "left": m.group("left"),
        "right": m.group("right"),
        "reason": m.group("reason"),
    }


# ---------- Main -------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--csv", required=True, help="rpcduel replay --csv mismatch file")
    ap.add_argument("--rpc-a", required=True)
    ap.add_argument("--rpc-b", required=True)
    ap.add_argument("--start", type=int, default=0, help="skip first N rows")
    ap.add_argument("--filter", default="", help="only show rows where detail contains SUBSTR")
    ap.add_argument("--context", type=int, default=6, help="sibling keys to render around the diverging key")
    ap.add_argument("--timeout", type=float, default=30.0)
    ap.add_argument("--no-color", action="store_true")
    ap.add_argument("--label-a", default="A")
    ap.add_argument("--label-b", default="B")
    args = ap.parse_args()

    global USE_COLOR
    if args.no_color:
        USE_COLOR = False

    rows: list[dict] = []
    with open(args.csv, newline="") as f:
        reader = csv.DictReader(f)
        for r in reader:
            if "mismatch" not in (r.get("category") or "").lower():
                continue
            if args.filter and args.filter not in (r.get("detail") or ""):
                continue
            rows.append(r)

    total = len(rows)
    if total == 0:
        print("No mismatch rows after filtering.")
        return 0

    print(bold(f"Loaded {total} mismatch rows from {args.csv}"))
    print(dim("Press Enter for next, q+Enter to quit, s+Enter to skip context fetch.\n"))

    for i, row in enumerate(rows, 1):
        if i <= args.start:
            continue

        method = row["method"]
        try:
            params = json.loads(row["params"])
        except json.JSONDecodeError as e:
            print(red(f"[{i}/{total}] cannot parse params JSON: {e}"))
            continue
        detail = parse_detail(row["detail"])

        print(bold(f"\n=== [{i}/{total}] {method} ===  category={row['category']}"))
        print(yellow("params: ") + json.dumps(params, ensure_ascii=False))
        if detail:
            print(yellow("path  : ") + detail["path"]
                  + dim(f"   ({detail['reason']})"))
            print(yellow(f"{args.label_a:>6}: ") + blue(detail["left"]))
            print(yellow(f"{args.label_b:>6}: ") + red(detail["right"]))
        else:
            print(yellow("detail: ") + (row.get("detail") or ""))

        # Allow user to skip live re-fetch.
        choice = input(dim("[Enter]=fetch & diff, s=skip fetch, q=quit > ")).strip().lower()
        if choice == "q":
            print(dim("bye."))
            return 0
        if choice == "s":
            continue

        # Re-fetch from both endpoints.
        try:
            ra = rpc_call(args.rpc_a, method, params, args.timeout)
        except Exception as e:
            ra = {"__fetch_error__": str(e)}
        try:
            rb = rpc_call(args.rpc_b, method, params, args.timeout)
        except Exception as e:
            rb = {"__fetch_error__": str(e)}

        # Locate the diverging sub-tree and render side-by-side.
        path_parts = parse_path(detail["path"]) if detail else []
        # Show the parent of the diverging key for context.
        parent_parts = path_parts[:-1] if path_parts else []

        sub_a = walk(ra, parent_parts)
        sub_b = walk(rb, parent_parts)

        # If the parent is huge (e.g. a long `calls[]` list), trim.
        sub_a = trim_large(sub_a, args.context)
        sub_b = trim_large(sub_b, args.context)

        ja = jdump(sub_a)
        jb = jdump(sub_b)

        header_l = f"{args.label_a}  $.{render_path(parent_parts)}"
        header_r = f"{args.label_b}  $.{render_path(parent_parts)}"
        side_by_side(ja, jb, header_l, header_r)

        # Re-print the focused diff line once more for clarity.
        if detail:
            print(yellow("\n→ field: ") + bold(detail["path"]) + "   "
                  + blue(detail["left"]) + dim("  vs  ") + red(detail["right"]))

        ans = input(dim("\n[Enter]=next, q=quit > ")).strip().lower()
        if ans == "q":
            print(dim("bye."))
            return 0

    print(bold("\nDone."))
    return 0


def render_path(parts: list) -> str:
    out = ""
    for p in parts:
        if isinstance(p, int):
            out += f"[{p}]"
        else:
            out += f".{p}" if out else p
    return out or "(root)"


def trim_large(v: Any, ctx: int) -> Any:
    """Trim large lists/dicts to keep side-by-side diffing readable."""
    if isinstance(v, list) and len(v) > ctx * 2:
        head = v[:ctx]
        tail = v[-ctx:]
        return head + [f"... <{len(v) - 2 * ctx} elided> ..."] + tail
    if isinstance(v, dict) and len(v) > 40:
        keys = list(v.keys())[:40]
        out = {k: v[k] for k in keys}
        out["__elided__"] = f"<{len(v) - 40} more keys>"
        return out
    return v


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        print()
        sys.exit(130)
