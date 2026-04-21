# `record` — one-shot capture into a bench scenario

`rpcduel record` collapses the two most common steps of the data-driven
workflow — chain scan (`dataset`) and scenario synthesis (`benchgen`) —
into a single command. The output is a `bench.json` file ready to feed
directly into `rpcduel bench --input`.

## Quick start

```bash
rpcduel record \
  --rpc https://rpc.example.com \
  --max-blocks 200 \
  --out bench.json

rpcduel bench --input bench.json --rpc https://rpc.a --rpc https://rpc.b \
  --duration 30s
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--rpc` | (required) | Source endpoint to record from |
| `--out` | `bench.json` | Output bench file path |
| `--from-block` / `--to-block` | latest − N | Block range to scan |
| `--max-blocks` | `200` | Cap the number of blocks scanned |
| `--max-txs` | (chain default) | Cap dataset transactions |
| `--max-accounts` | (chain default) | Cap dataset accounts |
| `--max-tx-per-account` | (chain default) | Per-account tx cap |
| `--methods` | _all_ | Comma-separated RPC method whitelist |
| `--sample` | `1.0` | Per-scenario downsample fraction (0..1) |
| `--trace-transaction` / `--trace-block` | `false` | Include trace methods in the scenarios |
| `--concurrency` | (chain default) | Goroutines used for the scan |
| `--seed` | `42` | RNG seed for reproducible sampling |
| `--chain` | (auto) | Chain hint (informational) |

## When to use `record` vs the two-step workflow

- **Use `record`** for ad-hoc capture, demos, and CI seeds where you
  just want a runnable `bench.json` quickly.
- **Use `dataset` + `benchgen`** when you need to inspect the dataset
  (`dataset inspect`), keep it under version control, or generate
  multiple bench files with different `--methods` / `--sample` knobs
  from a single scan.

## Tips

- `--methods eth_call,eth_getLogs` is great for stress-testing a
  specific RPC path observed in production traffic.
- `--sample 0.1` keeps the file small enough to commit to git while
  preserving every scenario (the sampler always keeps at least one
  request per scenario).
- Combine with [`dataset inspect`](./dataset.md#inspect-a-dataset-file)
  on the resulting file's underlying scan to estimate the request load
  before running.
