# `bench`

Benchmark one or more endpoints with concurrent load. Latencies are tracked with HDR histograms
(P50/P95/P99/P999) and the run can be either request-count- or duration-bound.

```
rpcduel bench [flags]
```

## Flags

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

## Examples

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

# Rate-limited fairness test against two nodes
rpcduel bench \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --rps 200 --warmup 5s --duration 60s
```

When `--input` is used, scenarios are sampled proportionally to their weights, creating realistic
mixed load. See [`benchgen`](/data-driven/benchgen) for how to generate that file from real data.

## See also

* [`benchgen`](/data-driven/benchgen) — generate weighted scenarios from a dataset
* [SLO thresholds](/advanced/thresholds) for failing CI on P95/P99/error rate
