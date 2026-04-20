# rpcduel examples

This directory contains small, runnable examples for the most common
`rpcduel` workflows. Replace the placeholder endpoints (`https://rpc-a`,
`https://rpc-b`) with real Ethereum-compatible JSON-RPC URLs before running.

## 1. Single call

Send one JSON-RPC request and pretty-print the response:

```bash
rpcduel call --rpc https://rpc-a --method eth_blockNumber
```

## 2. Diff two endpoints (single request)

```bash
rpcduel diff \
  --rpc https://rpc-a --rpc https://rpc-b \
  --method eth_getBlockByNumber --params '["latest", false]'
```

## 3. Diff a batch file

```bash
rpcduel diff \
  --rpc https://rpc-a --rpc https://rpc-b \
  --batch examples/batch-mini.json
```

## 4. Benchmark + diff (duel)

```bash
rpcduel duel \
  --rpc https://rpc-a --rpc https://rpc-b \
  --method eth_blockNumber \
  --concurrency 10 --requests 200
```

## 5. Build a dataset, then replay it

```bash
# 1. Build a small dataset from a live chain (≈ 50 accounts / 200 tx / 50 blocks).
rpcduel dataset \
  --rpc https://rpc-a \
  --accounts 50 --txs 200 --blocks 50 \
  --out examples/dataset-mini.json

# 2. Replay the dataset against two endpoints.
rpcduel replay \
  --dataset examples/dataset-mini.json \
  --rpc https://rpc-a --rpc https://rpc-b \
  --concurrency 8 \
  --csv examples/replay-diffs.csv

# 3. Generate weighted load scenarios from the dataset and run them.
rpcduel benchgen \
  --dataset examples/dataset-mini.json \
  --rpc https://rpc-a --rpc https://rpc-b \
  --concurrency 16 --requests 5000 \
  --csv examples/bench.csv
```

## 6. Global flags worth knowing

```bash
# Structured JSON logs at debug level.
rpcduel --log-level debug --log-format json call --rpc https://rpc-a --method eth_chainId

# Retry transient failures up to 3 times with exponential backoff.
rpcduel --retries 3 --retry-backoff 250ms diff \
  --rpc https://rpc-a --rpc https://rpc-b --method eth_blockNumber

# Pass an auth header to a gated provider.
rpcduel --header "Authorization: Bearer $RPC_TOKEN" \
  call --rpc https://rpc-a --method eth_blockNumber
```

See `rpcduel <command> --help` for the full per-command flag list.
