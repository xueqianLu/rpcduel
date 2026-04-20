# rpcduel

[![CI](https://github.com/xueqianLu/rpcduel/actions/workflows/ci.yml/badge.svg)](https://github.com/xueqianLu/rpcduel/actions/workflows/ci.yml)
[![Release](https://github.com/xueqianLu/rpcduel/actions/workflows/release.yml/badge.svg)](https://github.com/xueqianLu/rpcduel/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/xueqianLu/rpcduel.svg)](https://pkg.go.dev/github.com/xueqianLu/rpcduel)
[![Go Report Card](https://goreportcard.com/badge/github.com/xueqianLu/rpcduel)](https://goreportcard.com/report/github.com/xueqianLu/rpcduel)
[![License: MIT](https://img.shields.io/github/license/xueqianLu/rpcduel)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/xueqianLu/rpcduel?sort=semver)](https://github.com/xueqianLu/rpcduel/releases)

**rpcduel** is a high-performance CLI tool for comparing and benchmarking Ethereum JSON-RPC endpoints.  
It collects real on-chain data, runs response-consistency tests across multiple nodes, and generates realistic load-test scenarios — all from a single binary.

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Commands](#commands)
  - [call](#call)
  - [diff](#diff)
  - [bench](#bench)
  - [duel](#duel)
  - [dataset](#dataset)
  - [replay](#replay)
  - [benchgen](#benchgen)
- [Data-Driven Testing Workflow](#data-driven-testing-workflow)
- [Output Formats](#output-formats)
- [Dataset File Format](#dataset-file-format)
- [Architecture](#architecture)

---

## Features

| Capability | Description |
|---|---|
| **Direct RPC calls** | Invoke any Ethereum JSON-RPC method from the CLI without hand-writing `curl` commands |
| **Response diffing** | Deep JSON comparison with hex/decimal normalisation, field ignoring, and order-insensitive array comparison |
| **Benchmarking** | Concurrent load generation with QPS, avg/P95/P99 latency and error-rate reporting |
| **Duel mode** | Run diff and bench simultaneously against two endpoints |
| **On-chain dataset collection** | Scan a block range (high → low) via an Ethereum JSON-RPC endpoint using multiple concurrent goroutines and collect blocks, transactions, and accounts ranked by activity; per-account transaction lists are stored in the dataset for efficient replay |
| **Data-driven consistency tests** | Replay real chain data against two endpoints and classify every difference (`balance_mismatch`, `nonce_mismatch`, `tx_mismatch`, …) |
| **Scenario-driven load test** | Turn a dataset into weighted scenarios, optionally export them as a bench file, and run them directly with `benchgen`; per-scenario metrics (QPS, avg/P50/P95/P99/min/max latency, error rate) are written to a CSV report |
| **Archive-node awareness** | `missing trie node` / `state not found` errors are detected and excluded from diff counts |
| **Flexible output** | Human-readable text or machine-parseable JSON from every command |

---

## Installation

### Prebuilt binaries

Download the archive for your OS/arch from the [latest release](https://github.com/xueqianLu/rpcduel/releases),
extract it, and put `rpcduel` somewhere on your `PATH`.

### Docker

```bash
docker run --rm ghcr.io/xueqianlu/rpcduel:latest call --rpc https://rpc.example.com
```

### Go install

```bash
go install github.com/xueqianLu/rpcduel@latest
```

### Build from source

```bash
git clone https://github.com/xueqianLu/rpcduel.git
cd rpcduel
make build      # produces ./bin/rpcduel
```

Requires **Go 1.23+**.

---

## Commands

### Global flags

These flags apply to every subcommand:

| Flag | Default | Description |
|------|---------|-------------|
| `--log-level` | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |
| `--log-format` | `text` | Log format: `text` or `json` (structured output via `slog`). |
| `--retries` | `0` | Number of retries on network / 5xx / 408 / 429 failures. JSON-RPC application errors are not retried. |
| `--retry-backoff` | `200ms` | Initial exponential backoff between retries. |
| `--header` | (none) | Extra HTTP header sent with every RPC request. May be repeated. Accepts `Key: Value` or `Key=Value`. |
| `--user-agent` | `rpcduel/<version>` | Override the `User-Agent` header. |

Examples live in [`examples/`](./examples/README.md).

### `call`

Call any JSON-RPC method directly against a single endpoint. This is the fastest way to replace one-off `curl` commands when debugging or exploring node behavior.

```
rpcduel call [method] [param...] [flags]
```

When a method and params are provided positionally, `rpcduel` uses them directly. This makes one-off calls very convenient:

```bash
rpcduel call --rpc https://rpc.example.com eth_getBalance 0xa11111 latest
```

Positional params are parsed smartly:

- plain tokens like `latest`, `0xa11111`, addresses, and tx hashes stay as strings
- JSON literals like `true`, `false`, `null`, `123`, `{"k":1}`, and `[1,2]` are decoded as JSON values

To avoid ambiguity, positional params cannot be mixed with `--params` or `--params-file` in the same call.

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required)_ | Endpoint URL to call |
| `--method` | `eth_blockNumber` | JSON-RPC method |
| `--params` | `[]` | JSON-encoded params array |
| `--params-file` | | JSON file containing the params array; overrides `--params` |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

**Examples**

```bash
# Quick block number query
rpcduel call --rpc https://rpc.example.com

# Preferred shorthand: method + params as positional arguments
rpcduel call \
  --rpc https://rpc.example.com \
  eth_getBalance 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045 latest

# Flag-based form is still supported
rpcduel call \
  --rpc https://rpc.example.com \
  --method eth_getBalance \
  --params '["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]'

# Load params from a file to avoid shell escaping
rpcduel call \
  --rpc https://rpc.example.com \
  --method eth_getLogs \
  --params-file params.json \
  --output json
```

### `diff`

Compare the response of any JSON-RPC method across two or more endpoints.

```
rpcduel diff [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required, ≥2)_ | Endpoint URLs to compare |
| `--method` | `eth_blockNumber` | JSON-RPC method |
| `--params` | `[]` | JSON-encoded params array |
| `--input` | | JSON batch-request file `[{method, params}]` |
| `--repeat` | `1` | Repeat each request N times |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--ignore-order` | `false` | Treat arrays as unordered sets |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

**Examples**

```bash
# Compare eth_blockNumber across two nodes
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com

# Compare a specific block, ignoring the logsBloom field
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --method eth_getBlockByNumber \
  --params '["0x1000000", false]' \
  --ignore-field logsBloom

# Load a batch of requests from a file and output JSON
```json
# example requests.json
[
  {
    "method": "eth_getBalance",
    "params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]
  },
  {
    "method": "eth_getTransactionCount",
    "params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]
  }
]
```

```
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --input requests.json \
  --output json
```

---

### `bench`

Benchmark one or more endpoints with concurrent load.

```
rpcduel bench [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required, ≥1)_ | Endpoint URL(s) to benchmark |
| `--method` | `eth_blockNumber` | JSON-RPC method |
| `--params` | `[]` | JSON-encoded params array |
| `--input` | | Bench scenario file from `rpcduel benchgen` |
| `--concurrency` | `10` | Concurrent workers |
| `--requests` | `100` | Total requests (0 = use `--duration`) |
| `--duration` | | Run for a fixed time instead (e.g. `30s`) |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

**Examples**

```bash
# Quick single-method benchmark
rpcduel bench \
  --rpc https://rpc.example.com \
  --method eth_getBlockByNumber \
  --params '["latest", false]' \
  --concurrency 20 \
  --duration 60s

# Multi-scenario benchmark from a generated file (weighted dispatch)
rpcduel bench \
  --rpc https://rpc.example.com \
  --input bench.json \
  --concurrency 50 \
  --requests 10000
```

When `--input` is used with `--requests`, scenarios are sampled proportionally to their weights, creating realistic mixed load.

---

### `duel`

Run diff and bench **simultaneously** against exactly two endpoints — a single command that captures both consistency and performance.

```
rpcduel duel [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required, =2)_ | Exactly two endpoint URLs |
| `--method` | `eth_blockNumber` | JSON-RPC method |
| `--params` | `[]` | JSON-encoded params array |
| `--concurrency` | `10` | Concurrent workers |
| `--requests` | `100` | Total request pairs (0 = use `--duration`) |
| `--duration` | | Run for a fixed time (e.g. `30s`) |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--ignore-order` | `false` | Treat arrays as unordered sets |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

**Example**

```bash
rpcduel duel \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --method eth_getBalance \
  --params '["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]' \
  --concurrency 20 \
  --duration 30s
```

---

### `dataset`

Scan a block range **from high to low** via an Ethereum JSON-RPC endpoint and save a representative set of blocks, transactions, and accounts to a JSON file for use with `replay` and `benchgen`.

The scanner calls `eth_getBlockByNumber` (with full transaction objects) for every block in the range, collects non-empty blocks, extracts transactions, and ranks all addresses by the number of times they appear — all without requiring an external explorer.

```
rpcduel dataset [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required)_ | Ethereum JSON-RPC endpoint URL |
| `--from-block` | `0` | Start block, inclusive (0 = `toBlock − blocks×10`) |
| `--to-block` | `0` | End block, inclusive (0 = current chain head) |
| `--accounts` | `1000` | Max accounts to collect (sorted by observed tx count) |
| `--txs` | `1000` | Max transactions to collect |
| `--blocks` | `1000` | Max non-empty blocks to collect |
| `--max-tx-per-account` | `100` | Max transactions stored per account in the dataset (0 = unlimited) |
| `--concurrency` | `4` | Number of goroutines used to fetch blocks from the RPC endpoint |
| `--chain` | `ethereum` | Chain name written to dataset metadata |
| `--out` | `dataset.json` | Output file path |

**Example**

```bash
rpcduel dataset \
  --rpc https://rpc.example.com \
  --from-block 20000000 \
  --to-block 20001000 \
  --accounts 500 \
  --txs 500 \
  --blocks 200 \
  --chain ethereum \
  --out mainnet-dataset.json
```

Scanning stops early once all three collection limits (`--accounts`, `--txs`, `--blocks`) are satisfied. When `--to-block` is omitted, the current chain head is resolved automatically via `eth_blockNumber`.

Per-account transaction lists (up to `--max-tx-per-account` entries each) are embedded directly in each account record. This allows `replay` to query historical account state at the correct block numbers without re-fetching transaction lists at test time.

The exported JSON is deterministically ordered: accounts by tx count (descending), blocks by number (descending), and transactions by block number (ascending).

---

### `replay`

Load a dataset and run a full consistency test suite against two endpoints.  
Every account, transaction, and block in the dataset generates real RPC calls, and any response differences are classified and reported.

By default, `replay` covers the basic RPCs below. Trace RPCs can be enabled explicitly with flags when needed.

The old command name `diff-test` is still accepted as a compatibility alias.

- accounts → `eth_getBalance`, `eth_getTransactionCount`
- transactions → `eth_getTransactionByHash`, `eth_getTransactionReceipt`
- blocks → `eth_getBlockByNumber`
- optional trace RPCs → `debug_traceTransaction` (`--trace-transaction`), `debug_traceBlockByNumber` (`--trace-block`)

To avoid wasted work, duplicate tasks with the same **method + params** are automatically removed. Different methods for the same transaction or block are **not** treated as duplicates, so `eth_getTransactionByHash` and `debug_traceTransaction` will both run.

```
rpcduel replay [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, =2)_ | Exactly two endpoint URLs |
| `--max-tx-per-account` | `100` | Max transactions tested per account (0 = unlimited) |
| `--trace-transaction` | `false` | Also compare `debug_traceTransaction` for dataset transactions |
| `--trace-block` | `false` | Also compare `debug_traceBlockByNumber` for dataset blocks |
| `--only` | | Only run selected replay targets, e.g. `balance`, `transaction`, `block`, `trace` |
| `--concurrency` | `4` | Number of goroutines used to execute RPC calls |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |
| `--report` | | Write the report to this file (in addition to stdout) |
| `--csv` | | Write a CSV diff report (category, method, params, detail) to this file |

**Progress** is printed to stderr automatically as tasks complete — one line every 100 tasks and a final line at 100%.  Example:

```
Progress: 100/1000 tasks (10.0%)
Progress: 200/1000 tasks (20.0%)
...
Progress: 1000/1000 tasks (100.0%)
```

**Diff categories**

| Category | Trigger |
|---|---|
| `balance_mismatch` | `eth_getBalance` result differs |
| `nonce_mismatch` | `eth_getTransactionCount` result differs |
| `tx_mismatch` | `eth_getTransactionByHash` result differs |
| `receipt_mismatch` | `eth_getTransactionReceipt` result differs |
| `trace_mismatch` | `debug_traceTransaction` or `debug_traceBlockByNumber` result differs |
| `block_mismatch` | `eth_getBlockByNumber` result differs |
| `missing_data` | One endpoint returns `null`, the other does not |
| `rpc_error` | One endpoint returns an error, the other succeeds |
| `unsupported` _(not counted as diff)_ | Both endpoints return archive-node errors (`missing trie node`, `state not found`) |

Trace RPCs are often much heavier than standard RPCs and may not be enabled on every node, so they are disabled by default.

When `--only` is provided, replay only generates the selected directions. Supported values are:

- fine-grained: `balance`, `transaction_count`, `transaction_by_hash`, `transaction_receipt`, `block_by_number`, `trace_transaction`, `trace_block`
- aliases: `account`, `transaction`, `block`, `trace`

`--only` cannot be combined with `--trace-transaction` or `--trace-block`; if you want trace-only replay, use `--only trace` or `--only trace_transaction`.

**Example**

```bash
rpcduel replay \
  --dataset mainnet-dataset.json \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --max-tx-per-account 50 \
  --trace-transaction \
  --trace-block \
  --output json \
  --report replay-report.json \
  --csv replay-report.csv
```

```bash
# Only replay transaction-related checks
rpcduel replay \
  --dataset mainnet-dataset.json \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --only transaction
```

---

### `benchgen`

Generate weighted load-test scenarios from a dataset and **run them directly** against one or more endpoints.  
After the run, a performance summary is printed to stdout and an optional per-scenario CSV report can be written to a file. You can also export the generated scenario file with `--out` and reuse it later with `rpcduel bench --input`.

```
rpcduel benchgen [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, ≥1)_ | Endpoint URL(s) to benchmark |
| `--concurrency` | `10` | Number of concurrent workers |
| `--requests` | `1000` | Total requests to send (0 = use `--duration`) |
| `--duration` | | Run for a fixed time instead (e.g. `60s`) |
| `--timeout` | `30s` | Per-request timeout |
| `--trace-transaction` | `false` | Include the `debug_traceTransaction` scenario |
| `--trace-block` | `false` | Include the `debug_traceBlockByNumber` scenario |
| `--only` | | Only include selected scenario groups, e.g. `balance`, `transaction`, `block`, `logs`, `mixed_balance`, `trace` |
| `--out` | | Write the generated bench scenario file to this path |
| `--output` | `text` | `text` or `json` for the stdout summary |
| `--csv` | | Write a detailed per-scenario CSV report to this file |

**Generated scenarios**

| Scenario | Weight | Method |
|---|---|---|
| `balance` | 0.20 | `eth_getBalance` |
| `transaction_count` | 0.10 | `eth_getTransactionCount` |
| `transaction_by_hash` | 0.15 | `eth_getTransactionByHash` |
| `transaction_receipt` | 0.15 | `eth_getTransactionReceipt` |
| `block_by_number` | 0.10 | `eth_getBlockByNumber` |
| `get_logs` | 0.10 | `eth_getLogs` |
| `debug_trace_transaction` | 0.10 | `debug_traceTransaction` _(only with `--trace-transaction`)_ |
| `debug_trace_block` | 0.05 | `debug_traceBlockByNumber` _(only with `--trace-block`)_ |
| `mixed_balance` | 0.05 | `eth_getBalance` at shuffled historical block heights |

Requests are sampled from all enabled scenarios proportionally to their weights, producing realistic mixed traffic. In `--duration` mode, requests are sampled continuously at runtime instead of cycling a pre-built request pool.

Trace scenarios are disabled by default because they are often much heavier than standard RPCs and may not be supported by every node.

When `--only` is provided, benchgen limits generation and execution to the selected scenario groups. Supported values include:

- scenario names: `balance`, `transaction_count`, `transaction_by_hash`, `transaction_receipt`, `block_by_number`, `get_logs`, `mixed_balance`, `debug_trace_transaction`, `debug_trace_block`
- aliases: `account`, `transaction`, `block`, `logs`, `trace`, `trace_transaction`, `trace_block`

`--only` cannot be combined with `--trace-transaction` or `--trace-block`; use `--only trace` or `--only debug_trace_transaction` instead.

**CSV report columns**

`endpoint`, `scenario`, `total`, `errors`, `error_rate_pct`, `qps`, `avg_latency_ms`, `p50_latency_ms`, `p95_latency_ms`, `p99_latency_ms`, `min_latency_ms`, `max_latency_ms`

**Example**

```bash
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --concurrency 20 \
  --requests 5000 \
  --trace-transaction \
  --csv bench-report.csv
```

```bash
# Only benchmark logs + historical mixed balances
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --rpc https://node-a.example.com \
  --only logs,mixed_balance
```

```bash
# Export the generated scenario file without running it
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --trace-transaction \
  --out bench.json
```

---

## Data-Driven Testing Workflow

```
              Ethereum JSON-RPC endpoint
                         │
              rpcduel dataset --rpc …
                         │
                   dataset.json
                    ┌────┴────┐
                    │         │
             replay │         │ benchgen
                    │         │
                    ▼         ▼
              Consistency  Performance report
               report       + bench-report.csv
```

**Step 1 — Collect data**

```bash
rpcduel dataset \
  --rpc https://rpc.example.com \
  --from-block 20000000 --to-block 20001000 \
  --max-tx-per-account 100 \
  --out dataset.json
```

**Step 2 — Run consistency tests**

```bash
rpcduel replay \
  --dataset dataset.json \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --report replay-report.json \
  --csv replay-report.csv
```

Progress is printed to stderr as tasks complete. When finished, the summary is printed to stdout (or `--report` file) and a full CSV of all diffs is written to `--csv`.

**Step 3 — Run the load test directly**

```bash
rpcduel benchgen \
  --dataset dataset.json \
  --rpc https://node-a.example.com \
  --concurrency 20 \
  --requests 5000 \
  --csv bench-report.csv
```

Scenarios are generated internally from the dataset and executed immediately.  
The per-scenario performance summary is printed to stdout and a detailed CSV is written to `--csv`. If `--out` is provided, the generated bench file is also saved for reuse.

---

## Output Formats

All commands support `--output text` (default) and `--output json`.

**Text — call**

```
RPC Call Result
----------------------------------------
Endpoint: https://rpc.example.com
Method:   eth_getBalance
Latency:  12.345ms
Params:
  [
    "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
    "latest"
  ]
Result:
  "0x1a2b3c"
```

**JSON — call**

```json
{
  "endpoint": "https://rpc.example.com",
  "method": "eth_blockNumber",
  "params": [],
  "success": true,
  "latency_ms": 4.218,
  "result": "0x1312d00"
}
```

**Text — replay**

```
Replay Result
----------------------------------------
Accounts tested:     500
Transactions tested: 500
Blocks tested:       200
Total requests:      1700
Success rate:        99.8%
Unsupported:         2
Total diffs:         3

Diff summary:
  - balance_mismatch: 2
  - missing_data: 1

Sample diffs (up to 10):
  [balance_mismatch] eth_getBalance: [$.] 0x1a2b3c vs 0x1a2b3d (value mismatch)
```

**Text — bench**

```
Benchmark Result
----------------------------------------
Endpoint:   https://rpc.example.com
  Requests: 10000
  Errors:   12 (0.1%)
  QPS:      843.21
  Avg:      11.856ms
  P95:      24.301ms
  P99:      38.442ms
```

**JSON — replay**

```json
{
  "accounts_tested": 500,
  "transactions_tested": 500,
  "blocks_tested": 200,
  "total_requests": 1700,
  "success_rate": 0.998,
  "unsupported": 2,
  "total_diffs": 3,
  "diff_summary": {
    "balance_mismatch": 2,
    "missing_data": 1
  },
  "sample_diffs": [
    {
      "Category": "balance_mismatch",
      "Method": "eth_getBalance",
      "Params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "0x1312d00"],
      "Detail": "[$.] 0x1a2b3c vs 0x1a2b3d (value mismatch)"
    }
  ]
}
```

---

## Dataset File Format

```json
{
  "meta": {
    "chain": "ethereum",
    "rpc": "https://rpc.example.com",
    "generated_at": "2026-03-23T06:00:00Z"
  },
  "range": {
    "from": 20000000,
    "to": 20001000
  },
  "accounts": [
    {
      "address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
      "tx_count": 1234,
      "transactions": [
        {
          "hash": "0xabc123…",
          "block_number": 20000500,
          "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
          "to": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
        }
      ]
    }
  ],
  "transactions": [
    {
      "hash": "0xabc123…",
      "block_number": 20000500,
      "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
      "to": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
    }
  ],
  "blocks": [
    { "number": 20001000, "tx_count": 142 },
    { "number": 20000500, "tx_count": 87 }
  ]
}
```

**Sort order (applied automatically when saving):**

| Section | Order |
|---|---|
| `accounts` | `tx_count` descending (most active first) |
| `blocks` | `number` descending (newest first) |
| `transactions` | `block_number` ascending (chronological) |

Each account record includes a `transactions` list (up to `--max-tx-per-account` entries) containing the transactions that account participated in during the scan range. `replay` uses these stored block numbers to query historical state without re-fetching transaction data from the RPC node.

---

## Architecture

```
rpcduel/
├── cmd/                  CLI entry points (one file per subcommand)
│   ├── call.go
│   ├── diff.go
│   ├── bench.go
│   ├── duel.go
│   ├── dataset.go
│   ├── difftest.go
│   └── benchgen.go
└── internal/
    ├── rpc/              JSON-RPC HTTP client with latency measurement
    ├── diff/             Deep JSON comparison (hex normalisation, field ignoring, order)
    ├── bench/            Metrics collection (QPS, percentiles, error rate)
    ├── runner/           Concurrent worker pools (fixed count, duration, paired)
    ├── report/           Text & JSON report rendering
    ├── dataset/          Dataset types + Ethereum JSON-RPC chain scanner
    ├── benchgen/         Scenario generation & weighted request sampling
    └── replay/           Data-driven replay engine
```

**Key design decisions**

- **Hex normalisation** — `"0x1a"` and `"26"` are treated as equal by the diff engine, avoiding false positives from encoding differences.
- **Weighted dispatch** — `benchgen` assigns a weight to every scenario; `bench --input` samples requests proportionally, so realistic mixed traffic emerges without manual scripting.
- **Archive-node detection** — `replay` recognises `missing trie node` / `state not found` errors and marks those requests as `unsupported` rather than counting them as mismatches.
- **Graceful partial results** — network errors and API timeouts during dataset collection are logged as warnings; already-collected data is saved regardless.

---

## License

[MIT](LICENSE) 
