# `duel`

Run [`diff`](/commands/diff) and [`bench`](/commands/bench) **simultaneously** against exactly two
endpoints — a single command that captures both consistency and performance in the same run.

```
rpcduel duel [flags]
```

## Flags

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

## Example

```bash
rpcduel duel \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --method eth_getBalance \
  --params '["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]' \
  --concurrency 20 \
  --duration 30s
```

## See also

* [Data-driven workflow](/data-driven/workflow) — for a much richer two-endpoint comparison driven by real on-chain data
* [SLO thresholds](/advanced/thresholds) — gate `duel` on `p99_ms`, `error_rate`, and `diff_rate`
