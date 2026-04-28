# `replay`

Load a dataset and run a full consistency test suite against two endpoints. Every account,
transaction, and block in the dataset generates real RPC calls, and any response differences are
classified and reported.

```
rpcduel replay [flags]
```

::: tip
The old command name `diff-test` is still accepted as a compatibility alias.
:::

## Coverage

By default, `replay` covers the basic RPCs below. Trace RPCs are heavier and may not be supported
by every node, so they are opt-in.

* accounts → `eth_getBalance`, `eth_getTransactionCount`
* transactions → `eth_getTransactionByHash`, `eth_getTransactionReceipt`
* blocks → `eth_getBlockByNumber`
* optional trace RPCs → `debug_traceTransaction` (`--trace-transaction`), `debug_traceBlockByNumber` (`--trace-block`)

Duplicate tasks with the same **method + params** are automatically deduplicated. Different methods
for the same transaction or block are **not** considered duplicates, so
`eth_getTransactionByHash` and `debug_traceTransaction` will both run.

## Flags

| Flag | Default | Description |
|---|---|---|
| `--dataset` | `dataset.json` | Path to the dataset file |
| `--rpc` | _(required, =2)_ | Exactly two endpoint URLs |
| `--max-tx-per-account` | `100` | Max transactions tested per account (0 = unlimited) |
| `--trace-transaction` | `false` | Also compare `debug_traceTransaction` |
| `--trace-block` | `false` | Also compare `debug_traceBlockByNumber` |
| `--tracer` | `callTracer` | Tracer name passed to `debug_trace*` (e.g. `callTracer`, `prestateTracer`, `4byteTracer`, `noopTracer`, `muxTracer`, `flatCallTracer`). Use `default` to keep the node's built-in `structLogger`. |
| `--tracer-config` | _(none)_ | JSON object placed under `tracerConfig`, e.g. `'{"onlyTopCall":true}'` or `'{"diffMode":true}'`. |
| `--only` | | Only run selected targets, e.g. `balance`, `transaction`, `block`, `trace` |
| `--concurrency` | `4` | Number of goroutines used to execute RPC calls |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |
| `--report` | | Write the text/JSON report to this file (in addition to stdout) |
| `--csv` | | Write a CSV diff report (category, method, params, detail) |
| `--report-html` | | Self-contained HTML report (with category bar chart) |
| `--report-md` | | Markdown report (with per-category share column) |
| `--report-junit` | | JUnit XML report (replay metrics + per-category suite) |

## Diff categories

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
| `unsupported` | Both endpoints return archive-node errors (`missing trie node`, `state not found`). **Not counted as diff.** |

## `--only` targets

* fine-grained: `balance`, `transaction_count`, `transaction_by_hash`, `transaction_receipt`, `block_by_number`, `trace_transaction`, `trace_block`
* aliases: `account`, `transaction`, `block`, `trace`

`--only` cannot be combined with `--trace-transaction` or `--trace-block`. For trace-only replay,
use `--only trace` or `--only trace_transaction`.

## Progress

Progress is printed to **stderr** automatically as tasks complete — one line every 100 tasks and a
final line at 100%:

```
Progress: 100/1000 tasks (10.0%)
Progress: 200/1000 tasks (20.0%)
...
Progress: 1000/1000 tasks (100.0%)
```

## Examples

```bash
# Full consistency test with all reports
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

# Only replay transaction-related checks
rpcduel replay \
  --dataset mainnet-dataset.json \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --only transaction
```

## See also

* [SLO thresholds](/advanced/thresholds) — fail CI on `mismatch_rate`, `error_rate`, `max_mismatch`
* [Reports (HTML / MD / JUnit)](/advanced/reports)
* [`benchgen`](/data-driven/benchgen) — load test from the same dataset
