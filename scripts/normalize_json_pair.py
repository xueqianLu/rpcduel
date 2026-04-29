#!/usr/bin/env python3
# Copyright 2026 The rpcduel Authors
# SPDX-License-Identifier: Apache-2.0
"""
normalize_json_pair.py — normalise two JSON files for easier diffing.

For each input file the script:

  1. Recursively sorts dict keys into a stable (alphabetical) order.
  2. Where the same path exists in BOTH files but with different value
     types, tries to coerce both sides to the same type so a downstream
     `diff` (or `diff_json_files.py`) only flags genuine value
     differences.

The two outputs are written to disk. Pipe them into your favourite
side-by-side diff viewer:

    ./scripts/normalize_json_pair.py a.json b.json
    diff -u a.normalized.json b.normalized.json
    ./scripts/diff_json_files.py a.normalized.json b.normalized.json

Type-coercion rules applied at matching leaf paths:

    int   <-> numeric str        ("100", "0x64", "0o144", "0b1100100")
        Both sides become an int (canonical decimal form).
    float <-> numeric str        ("1.5")
        Both sides become a float.
    bool  <-> "true" / "false"   (case-insensitive)
        Both sides become a bool.
    null  <-> "null"             (case-insensitive)
        Both sides become null.

When the types match already, or when no coercion succeeds, the values
are left untouched. Coercion only happens at paths that exist in both
files; values present in only one file are passed through unchanged
(after key sorting).

Flags:
    -o, --out-a PATH    output path for normalised file A (default: a.normalized.json)
    -O, --out-b PATH    output path for normalised file B
    --stdout            print both files to stdout instead of writing to disk
                        (separated by '--- file ---' headers)
    --indent N          JSON indent (default 2; 0 = no indent)

Stdlib only.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys

_INT_RE = re.compile(r"^-?(?:0[xX][0-9a-fA-F]+|0[oO][0-7]+|0[bB][01]+|\d+)$")
_FLOAT_RE = re.compile(r"^-?\d+\.\d+(?:[eE][+-]?\d+)?$|^-?\d+[eE][+-]?\d+$")


# ---------- scalar coercion --------------------------------------------------

def _try_int(v):
    """Return int if v is an int or a string that parses as one, else None."""
    if isinstance(v, bool):
        return None
    if isinstance(v, int):
        return v
    if isinstance(v, str) and _INT_RE.match(v.strip()):
        try:
            return int(v.strip(), 0)
        except ValueError:
            return None
    return None


def _try_float(v):
    if isinstance(v, bool):
        return None
    if isinstance(v, float):
        return v
    if isinstance(v, int):
        return float(v)
    if isinstance(v, str) and _FLOAT_RE.match(v.strip()):
        try:
            return float(v.strip())
        except ValueError:
            return None
    return None


def _try_bool(v):
    if isinstance(v, bool):
        return v
    if isinstance(v, str):
        s = v.strip().lower()
        if s == "true":
            return True
        if s == "false":
            return False
    return None


def _try_null(v):
    if v is None:
        return True
    if isinstance(v, str) and v.strip().lower() == "null":
        return True
    return False


def coerce_pair(a, b):
    """If a and b have different types but a common interpretable form,
    return (a', b') in that common form. Otherwise return (a, b)."""
    if type(a) is type(b) and not isinstance(a, (dict, list)):
        return a, b

    # null <-> "null"
    if _try_null(a) and _try_null(b):
        return None, None

    # bool <-> "true"/"false"
    ba, bb = _try_bool(a), _try_bool(b)
    if ba is not None and bb is not None:
        return ba, bb

    # int <-> numeric str (preferred over float for hex-encoded gas values)
    ia, ib = _try_int(a), _try_int(b)
    if ia is not None and ib is not None:
        return ia, ib

    # float <-> numeric str
    fa, fb = _try_float(a), _try_float(b)
    if fa is not None and fb is not None:
        return fa, fb

    return a, b


# ---------- recursive normalisation ------------------------------------------

def sort_dict(v):
    """Recursively sort dict keys so output is stable."""
    if isinstance(v, dict):
        return {k: sort_dict(v[k]) for k in sorted(v.keys())}
    if isinstance(v, list):
        return [sort_dict(x) for x in v]
    return v


def normalize_pair(a, b):
    """Walk a and b in parallel, applying coerce_pair at matching leaves
    and sorting dict keys recursively. Returns (a', b')."""
    if isinstance(a, dict) and isinstance(b, dict):
        out_a, out_b = {}, {}
        for k in sorted(set(a.keys()) | set(b.keys())):
            if k in a and k in b:
                na, nb = normalize_pair(a[k], b[k])
                out_a[k] = na
                out_b[k] = nb
            elif k in a:
                out_a[k] = sort_dict(a[k])
            else:
                out_b[k] = sort_dict(b[k])
        return out_a, out_b

    if isinstance(a, list) and isinstance(b, list):
        out_a, out_b = [], []
        n = min(len(a), len(b))
        for i in range(n):
            na, nb = normalize_pair(a[i], b[i])
            out_a.append(na)
            out_b.append(nb)
        for i in range(n, len(a)):
            out_a.append(sort_dict(a[i]))
        for i in range(n, len(b)):
            out_b.append(sort_dict(b[i]))
        return out_a, out_b

    # At least one side is a leaf, or types disagree (dict vs list, etc.).
    return coerce_pair(a, b)


# ---------- main -------------------------------------------------------------

def default_out(path: str) -> str:
    base, ext = os.path.splitext(path)
    return f"{base}.normalized{ext or '.json'}"


def main() -> int:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("file_a")
    ap.add_argument("file_b")
    ap.add_argument("-o", "--out-a", default=None)
    ap.add_argument("-O", "--out-b", default=None)
    ap.add_argument("--stdout", action="store_true",
                    help="print both files to stdout instead of writing to disk")
    ap.add_argument("--indent", type=int, default=2)
    args = ap.parse_args()

    try:
        with open(args.file_a, encoding="utf-8") as f:
            a = json.load(f)
        with open(args.file_b, encoding="utf-8") as f:
            b = json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        print(f"failed to load: {e}", file=sys.stderr)
        return 2

    na, nb = normalize_pair(a, b)
    indent = args.indent if args.indent > 0 else None
    da = json.dumps(na, indent=indent, ensure_ascii=False, sort_keys=True)
    db = json.dumps(nb, indent=indent, ensure_ascii=False, sort_keys=True)

    if args.stdout:
        sys.stdout.write(f"--- {args.file_a} ---\n{da}\n")
        sys.stdout.write(f"--- {args.file_b} ---\n{db}\n")
        return 0

    out_a = args.out_a or default_out(args.file_a)
    out_b = args.out_b or default_out(args.file_b)
    with open(out_a, "w", encoding="utf-8") as f:
        f.write(da + "\n")
    with open(out_b, "w", encoding="utf-8") as f:
        f.write(db + "\n")
    print(f"wrote {out_a}")
    print(f"wrote {out_b}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        print()
        sys.exit(130)
