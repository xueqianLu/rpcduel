# rpcduel

[![CI](https://github.com/xueqianLu/rpcduel/actions/workflows/ci.yml/badge.svg)](https://github.com/xueqianLu/rpcduel/actions/workflows/ci.yml)
[![Release](https://github.com/xueqianLu/rpcduel/actions/workflows/release.yml/badge.svg)](https://github.com/xueqianLu/rpcduel/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/xueqianLu/rpcduel.svg)](https://pkg.go.dev/github.com/xueqianLu/rpcduel)
[![Go Report Card](https://goreportcard.com/badge/github.com/xueqianLu/rpcduel)](https://goreportcard.com/report/github.com/xueqianLu/rpcduel)
[![License: MIT](https://img.shields.io/github/license/xueqianLu/rpcduel)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/xueqianLu/rpcduel?sort=semver)](https://github.com/xueqianLu/rpcduel/releases)

**rpcduel** is a high-performance CLI for comparing and benchmarking Ethereum JSON-RPC endpoints.
A single binary lets you call any RPC, diff responses across nodes, run concurrent benchmarks, and — its signature feature — drive realistic consistency tests and load tests from on-chain data you collect yourself.

📖 **Full documentation:** <https://xueqianlu.github.io/rpcduel/>

---

## ✨ Why rpcduel?

> The **`dataset → replay / benchgen`** workflow is what sets rpcduel apart from generic benchmarking tools.
>
> You scan a real block range once, then reuse that dataset to run **data-driven consistency tests** (`replay`) and **realistic mixed-traffic load tests** (`benchgen`) against any pair of nodes — no scripting, no manual fixtures.

```
        Ethereum JSON-RPC endpoint
                   │
       rpcduel dataset --rpc …
                   │
              dataset.json
              ┌────┴─────┐
       replay │          │ benchgen
              ▼          ▼
       Consistency    Performance
         report      report + CSV
```

Jump straight to the [Data-Driven Workflow](#advanced-data-driven-testing) section, or read on for the basics.

---

## Table of Contents

- [Installation](#installation)
- [Basic Commands](#basic-commands)
  - [Global flags](#global-flags)
  - [`call`](#call)
  - [`diff`](#diff)
  - [`bench`](#bench)
  - [`duel`](#duel)
- [Advanced: Data-Driven Testing](#advanced-data-driven-testing) ⭐
  - [`dataset`](#dataset)
  - [`replay`](#replay)
  - [`benchgen`](#benchgen)
  - [End-to-end workflow](#end-to-end-workflow)
- [Advanced Features](#advanced-features)
  - [Configuration file](#configuration-file)
  - [SLO thresholds & CI gating](#slo-thresholds--ci-gating)
  - [Reports (HTML / Markdown / JUnit)](#reports-html--markdown--junit)
  - [Prometheus metrics & Pushgateway](#prometheus-metrics--pushgateway)
  - [`doctor` — endpoint capability check](#doctor--endpoint-capability-check)
  - [CI templates](#ci-templates)
  - [Shell completions & man pages](#shell-completions--man-pages)
- [Output Formats](#output-formats)
- [Dataset File Format](#dataset-file-format)
- [Architecture](#architecture)
- [License](#license)

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

## Basic Commands

The four basic commands cover everyday RPC ergonomics: invoke a method, compare responses across nodes, generate load, and do both at once.

### Global flags

These flags apply to every subcommand:

| Flag | Default | Description |
|------|---------|-------------|
| `--config`, `-c` | (none) | Path to an `rpcduel.yaml` config file. CLI flags always override config values. See [Configuration file](#configuration-file). |
| `--log-level` | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |
| `--log-format` | `text` | Log format: `text` or `json` (structured output via `slog`). |
| `--retries` | `0` | Retries on network / 5xx / 408 / 429 failures. JSON-RPC application errors are not retried. |
| `--retry-backoff` | `200ms` | Initial exponential backoff between retries. |
| `--header` | (none) | Extra HTTP header sent with every RPC request. May be repeated. Accepts `Key: Value` or `Key=Value`. |
| `--user-agent` | `rpcduel/<version>` | Override the `User-Agent` header. |
| `--insecure` | `false` | Skip TLS certificate verification on outbound HTTPS requests. **Development only.** |
| `--metrics-addr` | (disabled) | Expose Prometheus metrics at `/metrics` on the given address (e.g. `:9090`). |
| `--push-gateway` | (disabled) | Prometheus Pushgateway URL; metrics are pushed at command exit. |
| `--push-job` | `rpcduel` | Job label used when pushing. |
| `--push-label` | (none) | Extra Pushgateway grouping label, repeatable, in `key=value` form. |

Examples live in [`examples/`](./examples/README.md).

### `call`

Call any JSON-RPC method directly against a single endpoint. The fastest way to replace one-off `curl` commands when debugging or exploring node behavior.

```
rpcduel call [method] [param...] [flags]
```

When a method and params are provided positionally, `rpcduel` uses them directly:

```bash
rpcduel call --rpc https://rpc.example.com eth_getBalance 0xa11111 latest
```

Positional params are parsed smartly:

- plain tokens like `latest`, `0xa11111`, addresses, and tx hashes stay as strings
- JSON literals like `true`, `false`, `null`, `123`, `{"k":1}`, and `[1,2]` are decoded as JSON

To avoid ambiguity, positional params cannot be mixed with `--params` or `--params-file`.

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
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --input requests.json \
  --output json
```

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
| `--rps` | `0` | Token-bucket rate limit (requests/sec, 0 = unlimited) |
| `--warmup` | `0` | Warm-up phase whose samples are discarded (e.g. `5s`) |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

Latencies are tracked with HDR histograms (P50/P95/P99/P999).

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

## Advanced: Data-Driven Testing

This is the part that makes rpcduel different. Instead of asking you to invent fixtures or write traffic generators, rpcduel **scans real blocks once**, persists them to a `dataset.json`, and lets you replay that data as either a consistency test or a load test.

### `dataset`

Scan a block range **from high to low** via an Ethereum JSON-RPC endpoint and save a representative set of blocks, transactions, and accounts.

The scanner calls `eth_getBlockByNumber` (with full transaction objects) for every block in the range, collects non-empty blocks, extracts transactions, and ranks all addresses by activity — no external explorer required.

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
| `--max-tx-per-account` | `100` | Max transactions stored per account (0 = unlimited) |
| `--concurrency` | `4` | Number of goroutines used to fetch blocks |
| `--chain` | `ethereum` | Chain name written to dataset metadata |
| `--out` | `dataset.json` | Output file path |

```bash
rpcduel dataset \
  --rpc https://rpc.example.com \
  --from-block 20000000 \
  --to-block 20001000 \
  --accounts 500 --txs 500 --blocks 200 \
  --out mainnet-dataset.json
```

Scanning stops early once all three collection limits are satisfied. Per-account transaction lists are embedded so `replay` can query historical state without re-fetching tx data. Output is deterministically ordered.

### `replay`

Load a dataset and run a full consistency test suite against two endpoints. Every account, transaction, and block in the dataset generates real RPC calls, and any response differences are classified and reported.

By default, `replay` covers the basic RPCs below. Trace RPCs can be enabled explicitly.

- accounts → `eth_getBalance`, `eth_getTransactionCount`
- transactions → `eth_getTransactionByHash`, `eth_getTransactionReceipt`
- blocks → `eth_getBlockByNumber`
- optional trace RPCs → `debug_traceTransaction` (`--trace-transaction`), `debug_traceBlockByNumber` (`--trace-block`)

Duplicate tasks with the same **method + params** are automatically deduplicated.

```
rpcduel replay [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, =2)_ | Exactly two endpoint URLs |
| `--max-tx-per-account` | `100` | Max transactions tested per account (0 = unlimited) |
| `--trace-transaction` | `false` | Also compare `debug_traceTransaction` |
| `--trace-block` | `false` | Also compare `debug_traceBlockByNumber` |
| `--only` | | Limit to selected targets (`balance`, `transaction`, `block`, `trace`, …) |
| `--concurrency` | `4` | Number of goroutines used to execute RPC calls |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |
| `--report` | | Write the text/JSON report to this file (in addition to stdout) |
| `--csv` | | Write a CSV diff report (category, method, params, detail) to this file |
| `--report-html` | | Write a self-contained HTML report (with category bar chart) |
| `--report-md` | | Write a Markdown report |
| `--report-junit` | | Write a JUnit XML report (replay metrics + per-category suite) |

The old command name `diff-test` is still accepted as a compatibility alias.

**Progress** is printed to stderr automatically, one line every 100 tasks and a final 100% line.

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

When `--only` is provided, supported values are:

- fine-grained: `balance`, `transaction_count`, `transaction_by_hash`, `transaction_receipt`, `block_by_number`, `trace_transaction`, `trace_block`
- aliases: `account`, `transaction`, `block`, `trace`

`--only` cannot be combined with `--trace-transaction` or `--trace-block`.

```bash
rpcduel replay \
  --dataset mainnet-dataset.json \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --max-tx-per-account 50 \
  --trace-transaction --trace-block \
  --output json \
  --report replay-report.json \
  --csv replay-report.csv \
  --report-html replay.html \
  --report-junit replay.junit.xml
```

### `benchgen`

Generate weighted load-test scenarios from a dataset and **run them directly** against one or more endpoints. After the run, a performance summary is printed to stdout and an optional per-scenario CSV report is written. You can also export the scenario file with `--out` and reuse it later via `rpcduel bench --input`.

```
rpcduel benchgen [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, ≥1)_ | Endpoint URL(s) to benchmark |
| `--concurrency` | `10` | Concurrent workers |
| `--requests` | `1000` | Total requests (0 = use `--duration`) |
| `--duration` | | Run for a fixed time (e.g. `60s`) |
| `--timeout` | `30s` | Per-request timeout |
| `--trace-transaction` | `false` | Include `debug_traceTransaction` scenario |
| `--trace-block` | `false` | Include `debug_traceBlockByNumber` scenario |
| `--only` | | Only include selected scenario groups (`balance`, `transaction`, `block`, `logs`, `mixed_balance`, `trace`) |
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

Requests are sampled from all enabled scenarios proportionally to their weights. In `--duration` mode, requests are sampled continuously at runtime instead of cycling a pre-built request pool.

**CSV report columns:** `endpoint`, `scenario`, `total`, `errors`, `error_rate_pct`, `qps`, `avg_latency_ms`, `p50_latency_ms`, `p95_latency_ms`, `p99_latency_ms`, `min_latency_ms`, `max_latency_ms`.

```bash
# Run a realistic mixed load with trace scenarios and a CSV report
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --concurrency 20 --requests 5000 \
  --trace-transaction \
  --csv bench-report.csv

# Only benchmark logs + historical mixed balances
rpcduel benchgen --dataset mainnet-dataset.json \
  --rpc https://node-a.example.com \
  --only logs,mixed_balance

# Export the generated scenario file without running it
rpcduel benchgen --dataset mainnet-dataset.json \
  --trace-transaction --out bench.json
```

### End-to-end workflow

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

**Step 3 — Run the load test**

```bash
rpcduel benchgen \
  --dataset dataset.json \
  --rpc https://node-a.example.com \
  --concurrency 20 --requests 5000 \
  --csv bench-report.csv
```

---

## Advanced Features

### Configuration file

Pass `--config rpcduel.yaml` (or `-c`) to load defaults for any subcommand. CLI flags always win. Environment variables expand as `${VAR}` or `${VAR:-default}`; use `$$` for a literal `$`. See [`examples/rpcduel.yaml`](./examples/rpcduel.yaml) for a complete sample covering endpoints, defaults, per-command sections, thresholds, and report destinations.

### SLO thresholds & CI gating

The config file can declare SLO thresholds for every command. When any threshold is breached, rpcduel prints a `FAIL` summary to stderr and exits with code **2**, which makes CI fail loudly:

```yaml
thresholds:
  bench:
    p95_ms: 250
    p99_ms: 500
    error_rate: 0.01     # 1%
  duel:
    p99_ms: 500
    error_rate: 0.01
    diff_rate: 0.001     # 0.1% mismatch
  diff:
    diff_rate: 0.0
    max_diffs: 0
  replay:
    mismatch_rate: 0.001
    error_rate: 0.01
    max_mismatch: 5
```

### Reports (HTML / Markdown / JUnit)

`replay` (and the other commands when applicable) can emit machine- and human-friendly reports in addition to stdout:

| Flag | Format | Notes |
|---|---|---|
| `--report` | Text or JSON (matches `--output`) | Mirror of stdout to a file |
| `--csv` | CSV | Full per-diff detail (category, method, params, detail) |
| `--report-html` | Self-contained HTML | Includes a deterministic per-category bar chart |
| `--report-md` | Markdown | Includes a per-category share column |
| `--report-junit` | JUnit XML | One suite for threshold metrics + one suite per diff category |

Default destinations can be set under the `reports:` section of the config file.

### Prometheus metrics & Pushgateway

Pass `--metrics-addr :9090` to any command to expose a Prometheus-format metrics endpoint at `http://localhost:9090/metrics` while the command runs. The exporter publishes:

| Metric | Type | Labels |
|--------|------|--------|
| `rpcduel_requests_total` | counter | `endpoint`, `scenario`, `status` (`ok`/`error`) |
| `rpcduel_request_duration_seconds` | histogram | `endpoint`, `scenario` |
| `rpcduel_diffs_total` | counter | `endpoint_a`, `endpoint_b` |
| `rpcduel_replay_diffs_total` | counter | `category` |

`scenario` is the per-request tag from the bench scenario file when present, otherwise the JSON-RPC method name.

For short-lived runs (CI, cron) use the **Pushgateway** flags instead:

```bash
rpcduel replay \
  --dataset dataset.json \
  --rpc https://a.example.com --rpc https://b.example.com \
  --push-gateway http://pushgateway:9091 \
  --push-job nightly-replay \
  --push-label run_id=$GITHUB_RUN_ID \
  --push-label chain=mainnet
```

Metrics are pushed once at command exit.

### `doctor` — endpoint capability check

A fast pre-flight that probes a curated set of methods (`web3_clientVersion`, `eth_chainId`, `eth_blockNumber`, `eth_syncing`, `net_peerCount`, `eth_gasPrice`, plus anything you pass via `--probe`) against every endpoint in parallel and prints a capability matrix. Use it as a hard gate at the top of CI pipelines.

```
rpcduel doctor [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required, ≥1)_ | Endpoint URL(s) to probe |
| `--probe` | (none) | Extra JSON-RPC methods to probe, repeatable |
| `--timeout` | `10s` | Per-probe timeout |
| `--output` | `text` | `text` or `json` |
| `--fail-on` | `unreachable` | `never`, `unreachable`, or `any`. Failure exits with code **2**. |

```bash
rpcduel doctor \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --probe debug_traceTransaction \
  --fail-on any
```

### CI templates

Drop-in pipelines for GitHub Actions and GitLab CI live under [`examples/ci/`](./examples/ci/). They wire `doctor` + `diff` + `replay` with SLO thresholds and JUnit uploads, and include a README documenting the shape of a typical pipeline.

### Shell completions & man pages

Cobra ships completion scripts for bash, zsh, fish, and PowerShell:

```sh
source <(rpcduel completion bash)
rpcduel completion zsh > "${fpath[1]}/_rpcduel"
rpcduel completion fish > ~/.config/fish/completions/rpcduel.fish
```

Pre-built completion scripts and man pages are bundled in every release archive under `completions/` and `man/`.

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
  "range": { "from": 20000000, "to": 20001000 },
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

Each account record includes a `transactions` list (up to `--max-tx-per-account` entries) so `replay` can query historical state without re-fetching transaction data.

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
│   ├── difftest.go       (replay)
│   ├── benchgen.go
│   └── doctor.go
└── internal/
    ├── rpc/              JSON-RPC client (HTTP/WS/IPC) with latency measurement
    ├── diff/             Deep JSON comparison (hex normalisation, field ignoring, order)
    ├── bench/            HDR-histogram-backed metrics (QPS, percentiles, error rate)
    ├── runner/           Concurrent worker pools (fixed count, duration, paired)
    ├── report/           Text / JSON / HTML / Markdown / JUnit report rendering
    ├── dataset/          Dataset types + Ethereum JSON-RPC chain scanner
    ├── benchgen/         Scenario generation & weighted request sampling
    ├── replay/           Data-driven replay engine + diff classifier
    ├── doctor/           Endpoint capability probes
    ├── config/           rpcduel.yaml loader with env expansion
    ├── thresholds/       SLO threshold evaluator
    └── metrics/          Prometheus exporter + Pushgateway client
```

**Key design decisions**

- **Hex normalisation** — `"0x1a"` and `"26"` are equal in the diff engine, avoiding false positives from encoding differences.
- **Weighted dispatch** — `benchgen` assigns a weight to every scenario; sampling is proportional, so realistic mixed traffic emerges without manual scripting.
- **Archive-node detection** — `replay` recognises `missing trie node` / `state not found` errors and marks those requests as `unsupported` rather than counting them as mismatches.
- **Graceful partial results** — network errors and API timeouts during dataset collection are logged as warnings; already-collected data is saved regardless.
- **CI-first** — non-zero exit codes for SLO breaches, JUnit reports, doctor pre-flight, and Pushgateway support so rpcduel slots straight into existing pipelines.

---

## License

[MIT](LICENSE)
