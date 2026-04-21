# Data-Driven Testing Workflow ⭐

This is the part that makes rpcduel different.

> Instead of asking you to invent fixtures or write traffic generators, rpcduel **scans real blocks
> once**, persists them to a `dataset.json`, and lets you replay that data as either a consistency
> test or a load test.

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

## Why this matters

| Problem with synthetic load | How `dataset` solves it |
|---|---|
| Hand-crafted fixtures go stale | The dataset is regenerated from a real chain on demand |
| Synthetic addresses miss state-heavy code paths | Accounts are ranked by real on-chain activity |
| `eth_getLogs`-style queries need real block ranges | Real block numbers are recorded in the dataset |
| Trace methods need *valid* tx hashes / blocks | The same dataset row drives both `replay` and `benchgen` |

## End-to-end pipeline

### 1. Collect data

```bash
rpcduel dataset \
  --rpc https://rpc.example.com \
  --from-block 20000000 --to-block 20001000 \
  --max-tx-per-account 100 \
  --out dataset.json
```

[Full `dataset` reference →](/data-driven/dataset)

### 2. Run consistency tests

```bash
rpcduel replay \
  --dataset dataset.json \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --report replay-report.json \
  --csv replay-report.csv \
  --report-html replay.html \
  --report-junit replay.junit.xml
```

Every account, transaction, and block in the dataset becomes real RPC calls; differences are
classified into categories (`balance_mismatch`, `nonce_mismatch`, `tx_mismatch`,
`receipt_mismatch`, `trace_mismatch`, `block_mismatch`, `missing_data`, `rpc_error`).

[Full `replay` reference →](/data-driven/replay)

### 3. Run a realistic load test

```bash
rpcduel benchgen \
  --dataset dataset.json \
  --rpc https://node-a.example.com \
  --concurrency 20 --requests 5000 \
  --csv bench-report.csv
```

`benchgen` turns the dataset into weighted scenarios (`balance`, `transaction_by_hash`,
`get_logs`, `mixed_balance` historical state, …) and samples them proportionally to weights so
realistic mixed traffic emerges without manual scripting.

[Full `benchgen` reference →](/data-driven/benchgen)

## Tying it to CI

Combine with [SLO thresholds](/advanced/thresholds) and [JUnit reports](/advanced/reports) to fail
CI when the two nodes diverge or one regresses:

```bash
rpcduel --config rpcduel.yaml replay \
  --dataset dataset.json \
  --rpc $NODE_A --rpc $NODE_B \
  --report-junit replay.junit.xml \
  --push-gateway $PUSHGATEWAY_URL --push-job nightly-replay
```

See the [CI templates](/advanced/ci) for drop-in GitHub Actions and GitLab CI pipelines.
