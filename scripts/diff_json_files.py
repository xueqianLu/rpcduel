#!/usr/bin/env python3
# Copyright 2026 The rpcduel Authors
# SPDX-License-Identifier: Apache-2.0
"""
diff_json_files.py — open two JSON files, normalise key order, and walk the
side-by-side diff one screen at a time.

    ./scripts/diff_json_files.py a.json b.json

Both inputs are loaded, dict keys are recursively sorted into a stable order
(so superficial key-order differences don't pollute the diff), and the result
is rendered as two columns. Differing lines are highlighted and prefixed with
'≠'. Output paginates to fit the terminal — press Enter for the next page.

Controls (after each page):
    Enter   next page
    n       jump to next page that contains a diff
    q       quit
    a       show the rest in one go (no more paging)

Flags:
    --context N     when --only-diff is set, lines of context around each
                    diff hunk (default 3)
    --only-diff     skip pages that contain no differences
    --width W       force terminal width
    --no-color      disable ANSI colours
    --label-a STR / --label-b STR   column headers (default: file paths)

Stdlib only.
"""
from __future__ import annotations

import argparse
import json
import shutil
import sys

USE_COLOR = sys.stdout.isatty()


def c(code: str, s: str) -> str:
    return f"\033[{code}m{s}\033[0m" if USE_COLOR else s


def red(s: str) -> str:    return c("31", s)
def blue(s: str) -> str:   return c("34;1", s)
def yellow(s: str) -> str: return c("33;1", s)
def dim(s: str) -> str:    return c("2", s)
def bold(s: str) -> str:   return c("1", s)


def sort_keys(v):
    """Recursively sort dict keys; lists keep their original order."""
    if isinstance(v, dict):
        return {k: sort_keys(v[k]) for k in sorted(v.keys())}
    if isinstance(v, list):
        return [sort_keys(x) for x in v]
    return v


def load(path: str):
    with open(path, "r", encoding="utf-8") as f:
        return sort_keys(json.load(f))


def render(v) -> list[str]:
    return json.dumps(v, indent=2, ensure_ascii=False).splitlines() or [""]


def hpad(s: str, w: int) -> str:
    if len(s) >= w:
        return s[: w - 1] + "…"
    return s + " " * (w - len(s))


def page_size() -> tuple[int, int]:
    sz = shutil.get_terminal_size((120, 30))
    return max(60, sz.columns), max(10, sz.lines - 4)


def print_header(label_a: str, label_b: str, col: int) -> None:
    sep = dim(" | ")
    print(bold(blue(hpad(label_a, col))) + sep + bold(red(hpad(label_b, col))))
    print(dim("-" * col) + sep + dim("-" * col))


def print_row(la: str, lb: str, col: int) -> None:
    sep = dim(" | ")
    if la == lb:
        print("  " + hpad(la, col) + sep + hpad(lb, col))
    else:
        print(dim("≠ ") + blue(hpad(la, col)) + sep + red(hpad(lb, col)))


def main() -> int:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("file_a")
    ap.add_argument("file_b")
    ap.add_argument("--label-a", default=None)
    ap.add_argument("--label-b", default=None)
    ap.add_argument("--width", type=int, default=0)
    ap.add_argument("--context", type=int, default=3)
    ap.add_argument("--only-diff", action="store_true")
    ap.add_argument("--no-color", action="store_true")
    args = ap.parse_args()

    global USE_COLOR
    if args.no_color:
        USE_COLOR = False

    try:
        a = load(args.file_a)
        b = load(args.file_b)
    except (OSError, json.JSONDecodeError) as e:
        print(red(f"failed to load: {e}"), file=sys.stderr)
        return 2

    la_lines = render(a)
    lb_lines = render(b)
    n = max(len(la_lines), len(lb_lines))
    diff_flags = [
        (la_lines[i] if i < len(la_lines) else "") !=
        (lb_lines[i] if i < len(lb_lines) else "")
        for i in range(n)
    ]
    diff_total = sum(diff_flags)

    width, ph = page_size()
    if args.width:
        width = args.width
    col = (width - 3) // 2

    label_a = args.label_a or args.file_a
    label_b = args.label_b or args.file_b

    print(bold(f"Comparing {args.file_a}  vs  {args.file_b}"))
    print(dim(f"  {len(la_lines)} vs {len(lb_lines)} lines, {diff_total} diff lines"))
    print()

    show_all = False
    i = 0
    while i < n:
        # --only-diff: advance to the next page boundary that contains a diff.
        if args.only_diff and not show_all:
            # find next diff
            j = i
            while j < n and not diff_flags[j]:
                j += 1
            if j >= n:
                break
            # back off by --context lines
            i = max(0, j - args.context)

        end = min(n, i + ph)
        print_header(f"{label_a}  [{i+1}-{end}/{n}]",
                     f"{label_b}  [{i+1}-{end}/{n}]", col)
        for k in range(i, end):
            la = la_lines[k] if k < len(la_lines) else ""
            lb = lb_lines[k] if k < len(lb_lines) else ""
            print_row(la, lb, col)
        i = end

        if show_all or i >= n:
            continue

        prompt = dim(f"-- {i}/{n}  [Enter]=next, n=next diff, a=all, q=quit > ")
        try:
            ans = input(prompt).strip().lower()
        except EOFError:
            break
        if ans == "q":
            break
        if ans == "a":
            show_all = True
        elif ans == "n":
            # jump to next diff page
            j = i
            while j < n and not diff_flags[j]:
                j += 1
            if j >= n:
                print(dim("no more diffs."))
                break
            i = max(0, j - args.context)
        # default: keep going

    print(bold("\ndone."))
    return 0 if diff_total == 0 else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        print()
        sys.exit(130)
