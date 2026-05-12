#!/usr/bin/env python3
# Copyright 2026 The rpcduel Authors
# SPDX-License-Identifier: Apache-2.0
"""
verify_eth_call_vs_trace.py — sanity-check that historical `eth_call`
returns the same data as the real on-chain execution of the same
transaction, by comparing it against `debug_traceTransaction` with the
`callTracer`.

For each transaction in the dataset:

  1. eth_getTransactionByHash(hash)               -> tx detail
  2. eth_call([callObj, hex(blockNumber-1)])      -> returndata_a
  3. debug_traceTransaction(hash, {callTracer})   -> top-level `output`
                                                    field -> returndata_b
  4. compare returndata_a vs returndata_b         -> match / mismatch

A mismatch means historical `eth_call` is **not** behaving the same as
the in-block execution. Common legit reasons (not bugs in the node):

  - Same-block prior tx changed storage that this tx reads. eth_call at
    block N-1 sees the parent's snapshot, while the real execution sees
    the post-prior-tx snapshot. Cannot be fixed without
    `debug_traceCall` + state overrides.
  - The tx reverted on-chain. callTracer reports the revert reason in
    `output`, eth_call may return the revert data or raise an error;
    both treated as "reverted" here and compared as such.

Usage:
    python3 scripts/verify_eth_call_vs_trace.py \
        --dataset dataset.json \
        --rpc http://node:8545 \
        --output verify.csv \
        --concurrency 4 \
        --limit 1000

Stdlib only — no external dependencies.
"""

from __future__ import annotations

import argparse
import csv
import json
import sys
import threading
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import Any


# ---------------------------------------------------------------------------
# JSON-RPC client
# ---------------------------------------------------------------------------

_id_lock = threading.Lock()
_id_counter = 0


def _next_id() -> int:
    global _id_counter
    with _id_lock:
        _id_counter += 1
        return _id_counter


def rpc_call(url: str, method: str, params: list[Any], timeout: float) -> tuple[Any, str | None]:
    """Returns (result, error_message). On any failure returns (None, msg)."""
    body = json.dumps({
        "jsonrpc": "2.0",
        "id": _next_id(),
        "method": method,
        "params": params,
    }).encode()
    req = urllib.request.Request(
        url, data=body, headers={"Content-Type": "application/json"}, method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            data = resp.read()
    except urllib.error.HTTPError as e:
        # Try to read the JSON-RPC error body anyway.
        try:
            data = e.read()
        except Exception:
            return None, f"HTTP {e.code}: {e.reason}"
    except urllib.error.URLError as e:
        return None, f"URL error: {e.reason}"
    except Exception as e:  # noqa: BLE001
        return None, f"transport error: {e}"

    try:
        obj = json.loads(data)
    except Exception as e:  # noqa: BLE001
        return None, f"bad JSON response: {e}"

    if isinstance(obj, dict) and obj.get("error"):
        err = obj["error"]
        return None, f"rpc error code={err.get('code')} msg={err.get('message')}"
    if isinstance(obj, dict):
        return obj.get("result"), None
    return None, f"unexpected response shape: {type(obj).__name__}"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def hexint(v: Any) -> int | None:
    if v is None:
        return None
    if isinstance(v, int):
        return v
    if isinstance(v, str):
        s = v.strip().lower()
        if not s:
            return None
        try:
            return int(s, 16) if s.startswith("0x") else int(s, 10)
        except ValueError:
            return None
    return None


def normalize_returndata(v: Any) -> str:
    """Canonical lowercase hex representation. Empty / 0x / None -> '0x'."""
    if v is None:
        return "0x"
    if isinstance(v, str):
        s = v.strip().lower()
        if s in ("", "0x"):
            return "0x"
        if not s.startswith("0x"):
            s = "0x" + s
        return s
    return str(v).lower()


def build_call_object(tx: dict[str, Any]) -> dict[str, Any] | None:
    """Build an eth_call argument from an eth_getTransactionByHash result.
    Returns None for contract creations (no `to`)."""
    to = tx.get("to")
    if not to:
        return None
    obj: dict[str, Any] = {"to": to}
    # Caller — important: many contracts gate behaviour on msg.sender.
    if tx.get("from"):
        obj["from"] = tx["from"]
    # Calldata — both `input` and `data` appear in different node versions.
    data = tx.get("input") or tx.get("data") or ""
    if data and data != "0x":
        obj["data"] = data
    if tx.get("value") and tx["value"] != "0x0":
        obj["value"] = tx["value"]
    if tx.get("gas"):
        obj["gas"] = tx["gas"]
    # Pricing fields — provide whichever the tx had so the simulation
    # uses the same fee market as the real execution.
    if tx.get("gasPrice") and tx["gasPrice"] != "0x0":
        obj["gasPrice"] = tx["gasPrice"]
    if tx.get("maxFeePerGas"):
        obj["maxFeePerGas"] = tx["maxFeePerGas"]
    if tx.get("maxPriorityFeePerGas"):
        obj["maxPriorityFeePerGas"] = tx["maxPriorityFeePerGas"]
    return obj


# ---------------------------------------------------------------------------
# Per-tx verification
# ---------------------------------------------------------------------------

@dataclass
class Outcome:
    hash: str
    block: int
    status: str  # match / mismatch / skip / error
    eth_call: str
    trace: str
    detail: str


def verify_one(url: str, tx_hash: str, dataset_block: int, timeout: float) -> Outcome:
    # 1. fetch tx detail
    tx, err = rpc_call(url, "eth_getTransactionByHash", [tx_hash], timeout)
    if err:
        return Outcome(tx_hash, dataset_block, "error", "", "", f"getTransactionByHash: {err}")
    if tx is None:
        return Outcome(tx_hash, dataset_block, "skip", "", "", "tx not found on rpc")
    block_num = hexint(tx.get("blockNumber"))
    if block_num is None or block_num <= 0:
        return Outcome(tx_hash, dataset_block, "skip", "", "", "tx has no blockNumber (pending?)")

    # Only the first tx in a block is meaningfully comparable: for any
    # later tx, the real on-chain execution sees state mutated by all
    # prior same-block txs, while eth_call at block N-1 sees only the
    # parent's snapshot. Skip the rest to avoid false mismatches.
    tx_index = hexint(tx.get("transactionIndex"))
    if tx_index is None:
        return Outcome(tx_hash, block_num, "skip", "", "", "tx has no transactionIndex")
    if tx_index != 0:
        return Outcome(tx_hash, block_num, "skip", "", "",
                       f"not first tx in block (transactionIndex={tx_index})")

    call_obj = build_call_object(tx)
    if call_obj is None:
        return Outcome(tx_hash, block_num, "skip", "", "", "contract creation (no `to`)")

    parent = hex(block_num - 1)

    # 2. eth_call at parent block
    call_result, call_err = rpc_call(url, "eth_call", [call_obj, parent], timeout)
    if call_err:
        # Many nodes raise on revert; capture the message verbatim so
        # we can compare it against trace's revert reason below.
        call_returndata = "ERROR:" + call_err
    else:
        call_returndata = normalize_returndata(call_result)

    # 3. debug_traceTransaction with callTracer
    trace_result, trace_err = rpc_call(
        url, "debug_traceTransaction", [tx_hash, {"tracer": "callTracer"}], timeout,
    )
    if trace_err:
        return Outcome(tx_hash, block_num, "error", call_returndata, "",
                       f"debug_traceTransaction: {trace_err}")
    if not isinstance(trace_result, dict):
        return Outcome(tx_hash, block_num, "error", call_returndata, "",
                       f"trace returned non-object: {type(trace_result).__name__}")

    trace_output = trace_result.get("output", "")
    trace_returndata = normalize_returndata(trace_output)
    trace_error = trace_result.get("error")  # set on revert/oog/etc.

    # 4. compare
    # Reverts are tricky: callTracer reports the revert reason in `output`
    # AND sets `error`. eth_call reports revert via JSON-RPC error with the
    # same revert data embedded in its `data` field (geth) or message
    # (other clients). We treat "both reverted with same data" as match.
    if trace_error:
        # Real tx reverted. Did eth_call also revert?
        is_call_revert = call_returndata.startswith("ERROR:")
        if is_call_revert and trace_returndata == "0x":
            return Outcome(tx_hash, block_num, "match", call_returndata, trace_returndata,
                           f"both reverted ({trace_error})")
        if is_call_revert:
            # Check whether the call's error message embeds the revert data.
            if trace_returndata != "0x" and trace_returndata[2:] in call_returndata.lower():
                return Outcome(tx_hash, block_num, "match", call_returndata, trace_returndata,
                               f"both reverted with same data ({trace_error})")
            return Outcome(tx_hash, block_num, "mismatch", call_returndata, trace_returndata,
                           f"both reverted but data differs ({trace_error})")
        # eth_call did NOT revert but tx did on-chain → real divergence.
        return Outcome(tx_hash, block_num, "mismatch", call_returndata, trace_returndata,
                       f"trace reverted ({trace_error}) but eth_call succeeded")

    # Success path on-chain.
    if call_returndata.startswith("ERROR:"):
        return Outcome(tx_hash, block_num, "mismatch", call_returndata, trace_returndata,
                       "eth_call errored but real tx succeeded")
    if call_returndata == trace_returndata:
        return Outcome(tx_hash, block_num, "match", call_returndata, trace_returndata, "")
    return Outcome(tx_hash, block_num, "mismatch", call_returndata, trace_returndata,
                   "returndata differs")


# ---------------------------------------------------------------------------
# Dataset loading
# ---------------------------------------------------------------------------

def load_tx_list(path: str) -> list[tuple[str, int]]:
    with open(path, "r", encoding="utf-8") as f:
        ds = json.load(f)
    out: list[tuple[str, int]] = []
    seen: set[str] = set()
    for tx in ds.get("transactions", []) or []:
        h = (tx.get("hash") or "").lower()
        if not h or h in seen:
            continue
        seen.add(h)
        out.append((h, int(tx.get("block_number") or 0)))
    return out


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--dataset", required=True, help="Path to dataset JSON (rpcduel format)")
    p.add_argument("--rpc", required=True, help="JSON-RPC endpoint (must support debug_traceTransaction)")
    p.add_argument("--output", default="verify_eth_call_vs_trace.csv", help="CSV output path")
    p.add_argument("--concurrency", type=int, default=4, help="Parallel workers (default 4)")
    p.add_argument("--timeout", type=float, default=30.0, help="Per-request timeout, seconds")
    p.add_argument("--limit", type=int, default=0, help="Max transactions to process (0 = all)")
    p.add_argument("--show-mismatches", action="store_true",
                   help="Print each mismatch to stderr as it happens")
    args = p.parse_args()

    txs = load_tx_list(args.dataset)
    if args.limit > 0:
        txs = txs[: args.limit]
    total = len(txs)
    if total == 0:
        print("dataset has no transactions", file=sys.stderr)
        return 1
    print(f"verifying {total} transactions against {args.rpc} (concurrency={args.concurrency})",
          file=sys.stderr)

    counts = {"match": 0, "mismatch": 0, "skip": 0, "error": 0}
    start = time.time()

    with open(args.output, "w", encoding="utf-8", newline="") as f:
        w = csv.writer(f)
        w.writerow(["tx_hash", "block_number", "status", "eth_call_returndata",
                    "trace_returndata", "detail"])

        with ThreadPoolExecutor(max_workers=max(1, args.concurrency)) as ex:
            futures = [ex.submit(verify_one, args.rpc, h, b, args.timeout) for h, b in txs]
            done = 0
            for fut in as_completed(futures):
                o = fut.result()
                counts[o.status] = counts.get(o.status, 0) + 1
                # Truncate huge return data in CSV to keep file usable.
                w.writerow([o.hash, o.block, o.status,
                            o.eth_call[:512], o.trace[:512], o.detail])
                done += 1
                if args.show_mismatches and o.status == "mismatch":
                    print(f"MISMATCH {o.hash} block={o.block}\n"
                          f"  eth_call: {o.eth_call[:200]}\n"
                          f"  trace:    {o.trace[:200]}\n"
                          f"  detail:   {o.detail}",
                          file=sys.stderr)
                if done % 100 == 0 or done == total:
                    elapsed = time.time() - start
                    rate = done / elapsed if elapsed > 0 else 0.0
                    print(f"  progress: {done}/{total} match={counts['match']} "
                          f"mismatch={counts['mismatch']} skip={counts['skip']} "
                          f"error={counts['error']} ({rate:.1f}/s)",
                          file=sys.stderr)

    elapsed = time.time() - start
    print("", file=sys.stderr)
    print(f"== summary ({elapsed:.1f}s) ==", file=sys.stderr)
    for k in ("match", "mismatch", "skip", "error"):
        print(f"  {k:9s} {counts[k]:>8d}", file=sys.stderr)
    print(f"  {'total':9s} {total:>8d}", file=sys.stderr)
    print(f"CSV written to: {args.output}", file=sys.stderr)

    # Non-zero exit if any real mismatch — useful in CI.
    return 2 if counts["mismatch"] > 0 else 0


if __name__ == "__main__":
    sys.exit(main())
