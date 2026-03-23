# rpcduel

**rpcduel** is a high-performance CLI tool for comparing and benchmarking Ethereum JSON-RPC endpoints.  
It collects real on-chain data, runs response-consistency tests across multiple nodes, and generates realistic load-test scenarios — all from a single binary.

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Commands](#commands)
  - [diff](#diff)
  - [bench](#bench)
  - [duel](#duel)
  - [dataset](#dataset)
  - [diff-test](#diff-test)
  - [benchgen](#benchgen)
- [Data-Driven Testing Workflow](#data-driven-testing-workflow)
- [Output Formats](#output-formats)
- [Dataset File Format](#dataset-file-format)
- [Bench Scenario File Format](#bench-scenario-file-format)
- [Architecture](#architecture)

---

## Features

| Capability | Description |
|---|---|
| **Response diffing** | Deep JSON comparison with hex/decimal normalisation, field ignoring, and order-insensitive array comparison |
| **Benchmarking** | Concurrent load generation with QPS, avg/P95/P99 latency and error-rate reporting |
| **Duel mode** | Run diff and bench simultaneously against two endpoints |
| **On-chain dataset collection** | Scan a block range (high → low) via an Ethereum JSON-RPC endpoint using multiple concurrent goroutines and collect blocks, transactions, and accounts ranked by activity |
| **Data-driven consistency tests** | Replay real chain data against two endpoints and classify every difference (`balance_mismatch`, `nonce_mismatch`, `tx_mismatch`, …) |
| **Scenario generation** | Turn a dataset into a weighted, multi-scenario bench file ready for `bench --input` |
| **Archive-node awareness** | `missing trie node` / `state not found` errors are detected and excluded from diff counts |
| **Flexible output** | Human-readable text or machine-parseable JSON from every command |

---

## Installation

```bash
go install github.com/xueqianLu/rpcduel@latest
```

Or build from source:

```bash
git clone https://github.com/xueqianLu/rpcduel.git
cd rpcduel
go build -o rpcduel .
```

Requires **Go 1.21+**.

---

## Commands

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

Scan a block range **from high to low** via an Ethereum JSON-RPC endpoint and save a representative set of blocks, transactions, and accounts to a JSON file for use with `diff-test` and `benchgen`.

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

---

### `diff-test`

Load a dataset and run a full consistency test suite against two endpoints.  
Every account, transaction, and block in the dataset generates real RPC calls, and any response differences are classified and reported.

```
rpcduel diff-test [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, =2)_ | Exactly two endpoint URLs |
| `--max-tx-per-account` | `100` | Max transactions tested per account (0 = unlimited) |
| `--concurrency` | `4` | Number of goroutines used to execute RPC calls |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

**Diff categories**

| Category | Trigger |
|---|---|
| `balance_mismatch` | `eth_getBalance` result differs |
| `nonce_mismatch` | `eth_getTransactionCount` result differs |
| `tx_mismatch` | `eth_getTransactionByHash` result differs |
| `receipt_mismatch` | `eth_getTransactionReceipt` result differs |
| `block_mismatch` | `eth_getBlockByNumber` result differs |
| `missing_data` | One endpoint returns `null`, the other does not |
| `rpc_error` | One endpoint returns an error, the other succeeds |
| `unsupported` _(not counted as diff)_ | Both endpoints return archive-node errors (`missing trie node`, `state not found`) |

**Example**

```bash
rpcduel diff-test \
  --dataset mainnet-dataset.json \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --max-tx-per-account 50 \
  --output json
```

---

### `benchgen`

Generate a multi-scenario benchmark file from a dataset.  
The output `bench.json` can be passed directly to `rpcduel bench --input`.

```
rpcduel benchgen [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--out` | `bench.json` | Output benchmark scenario file |

**Generated scenarios**

| Scenario | Weight | Method |
|---|---|---|
| `balance` | 0.20 | `eth_getBalance` |
| `transaction_count` | 0.10 | `eth_getTransactionCount` |
| `transaction_by_hash` | 0.15 | `eth_getTransactionByHash` |
| `transaction_receipt` | 0.15 | `eth_getTransactionReceipt` |
| `block_by_number` | 0.10 | `eth_getBlockByNumber` |
| `get_logs` | 0.10 | `eth_getLogs` |
| `debug_trace_transaction` | 0.10 | `debug_traceTransaction` |
| `debug_trace_block` | 0.05 | `debug_traceBlockByNumber` |
| `mixed_balance` | 0.05 | `eth_getBalance` (shuffled accounts) |

**Example**

```bash
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --out mainnet-bench.json
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
          diff-test │         │ benchgen
                    │         │
                    ▼         ▼
              Consistency  bench.json
               report      │
                         bench --input bench.json
                            │
                      Performance report
```

**Step 1 — Collect data**

```bash
rpcduel dataset \
  --rpc https://rpc.example.com \
  --from-block 20000000 --to-block 20001000 \
  --out dataset.json
```

**Step 2 — Run consistency tests**

```bash
rpcduel diff-test \
  --dataset dataset.json \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com
```

**Step 3 — Generate load-test scenarios**

```bash
rpcduel benchgen --dataset dataset.json --out bench.json
```

**Step 4 — Run the benchmark**

```bash
rpcduel bench \
  --rpc https://node-a.example.com \
  --input bench.json \
  --concurrency 50 \
  --duration 60s
```

---

## Output Formats

All commands support `--output text` (default) and `--output json`.

**Text — diff-test**

```
Diff-Test Result
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

**JSON — diff-test**

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
    { "address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "tx_count": 1234 }
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
    { "number": 20000500, "tx_count": 142 }
  ]
}
```

---

## Bench Scenario File Format

```json
{
  "version": "1",
  "scenarios": [
    {
      "name": "balance",
      "weight": 0.20,
      "requests": [
        {
          "method": "eth_getBalance",
          "params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]
        }
      ]
    }
  ]
}
```

Weights are used by `rpcduel bench --input` to sample requests proportionally across scenarios, producing realistic mixed traffic.

---

## Architecture

```
rpcduel/
├── cmd/                  CLI entry points (one file per subcommand)
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
    └── replay/           Data-driven diff-test engine
```

**Key design decisions**

- **Hex normalisation** — `"0x1a"` and `"26"` are treated as equal by the diff engine, avoiding false positives from encoding differences.
- **Weighted dispatch** — `benchgen` assigns a weight to every scenario; `bench --input` samples requests proportionally, so realistic mixed traffic emerges without manual scripting.
- **Archive-node detection** — `diff-test` recognises `missing trie node` / `state not found` errors and marks those requests as `unsupported` rather than counting them as mismatches.
- **Graceful partial results** — network errors and API timeouts during dataset collection are logged as warnings; already-collected data is saved regardless.

---

## License

[MIT](LICENSE) 
