# `benchgen`

Generate weighted load-test scenarios from a dataset and **run them directly** against one or more
endpoints. After the run, a performance summary is printed to stdout and an optional per-scenario
CSV report can be written. You can also export the generated scenario file with `--out` and reuse
it later via `rpcduel bench --input`.

```
rpcduel benchgen [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, ≥1)_ | Endpoint URL(s) to benchmark |
| `--concurrency` | `10` | Concurrent workers |
| `--requests` | `1000` | Total requests (0 = use `--duration`) |
| `--duration` | | Run for a fixed time instead (e.g. `60s`) |
| `--timeout` | `30s` | Per-request timeout |
| `--trace-transaction` | `false` | Include `debug_traceTransaction` scenario |
| `--trace-block` | `false` | Include `debug_traceBlockByNumber` scenario |
| `--tracer` | `callTracer` | Tracer name passed to `debug_trace*` (e.g. `callTracer`, `prestateTracer`, `4byteTracer`, `noopTracer`, `muxTracer`, `flatCallTracer`). Use `default` to keep the node's built-in `structLogger`. |
| `--tracer-config` | _(none)_ | JSON object placed under `tracerConfig`, e.g. `'{"onlyTopCall":true}'` (callTracer) or `'{"diffMode":true}'` (prestateTracer). |
| `--only` | | Only include selected scenario groups |
| `--out` | | Write the generated bench scenario file to this path |
| `--output` | `text` | `text` or `json` for the stdout summary |
| `--csv` | | Write a detailed per-scenario CSV report to this file |

## Generated scenarios

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

Requests are sampled from all enabled scenarios proportionally to their weights, producing
realistic mixed traffic. In `--duration` mode, requests are sampled continuously at runtime
instead of cycling a pre-built request pool.

## `--only` groups

* scenario names: `balance`, `transaction_count`, `transaction_by_hash`, `transaction_receipt`, `block_by_number`, `get_logs`, `mixed_balance`, `debug_trace_transaction`, `debug_trace_block`
* aliases: `account`, `transaction`, `block`, `logs`, `trace`, `trace_transaction`, `trace_block`

`--only` cannot be combined with `--trace-transaction` or `--trace-block`.

## CSV report columns

`endpoint`, `scenario`, `total`, `errors`, `error_rate_pct`, `qps`, `avg_latency_ms`,
`p50_latency_ms`, `p95_latency_ms`, `p99_latency_ms`, `min_latency_ms`, `max_latency_ms`

## Examples

```bash
# Full mixed-traffic load test with trace scenarios + per-scenario CSV
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --concurrency 20 --requests 5000 \
  --trace-transaction \
  --csv bench-report.csv

# Only benchmark logs + historical mixed balances
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --rpc https://node-a.example.com \
  --only logs,mixed_balance

# Export the generated scenario file without running it
rpcduel benchgen \
  --dataset mainnet-dataset.json \
  --trace-transaction \
  --out bench.json
# Re-run later with `rpcduel bench --input bench.json`
```

## See also

* [`bench`](/commands/bench) — run a previously-exported scenario file
* [SLO thresholds](/advanced/thresholds) — fail CI on `p95_ms`, `p99_ms`, `error_rate`
