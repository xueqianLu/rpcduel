# Resuming long runs

Long-running `replay` and `bench` jobs against busy chains can take
minutes or hours. `rpcduel` lets you checkpoint progress to a state
file and resume after a crash, network glitch, or Ctrl+C.

## Replay

```bash
# Start a long replay; checkpoint every 100 completed task keys.
rpcduel replay \
  --dataset dataset.json \
  --rpc https://rpc.a --rpc https://rpc.b \
  --state-file replay.state.json \
  --state-interval 100

# … later, after Ctrl+C or a crash …
rpcduel replay \
  --dataset dataset.json \
  --rpc https://rpc.a --rpc https://rpc.b \
  --state-file replay.state.json \
  --resume
```

What's persisted:

- The set of completed `(method, params)` task keys (canonicalized
  JSON), so resume dedupes against any task that already produced a
  result.
- Aggregate counters: total / success requests, unsupported.
- All discovered diffs.

What is **not** persisted:

- Per-endpoint timings — replay's role is correctness, not perf.
- Tasks that were in-flight at interrupt time. They will be re-run
  on resume; replay is idempotent so this is safe.

The state file is written every `--state-interval` completed tasks
**and** on graceful shutdown (Ctrl+C / SIGTERM). It can be safely
deleted to start over.

## Bench

`bench --resume` is intentionally scoped to **single-method
`--requests N` mode** — that is, runs that do not use `--input`
(scenario file) or `--duration`. This is the mode where “continue
where we left off” has clear semantics: there is a target request
count, and we resume with the remaining count split across endpoints.

```bash
rpcduel bench \
  --rpc https://rpc.a --rpc https://rpc.b \
  --method eth_blockNumber \
  --requests 100000 \
  --concurrency 50 \
  --state-file bench.state.json \
  --state-interval 200

# Resume after interrupt:
rpcduel bench \
  --rpc https://rpc.a --rpc https://rpc.b \
  --method eth_blockNumber \
  --requests 100000 \
  --concurrency 50 \
  --state-file bench.state.json \
  --resume
```

What's persisted per endpoint:

- Compressed HDR histogram snapshot (re-imported on resume so all
  percentiles remain accurate).
- Total / errors / min / max latency.
- Original `start_time` so QPS reflects the cumulative window.

Resume skips warmup automatically and continues with
`requests − sum(per_endpoint.total)` remaining requests.

## Limitations & gotchas

- For `replay`, resume requires the same `--rpc` endpoint pair as the
  original run (we record them in the state file and refuse to mix).
- For `bench`, resume requires the same `--method` and `--params`
  string as the original run.
- State files are JSON. They are safe to read and inspect, but should
  not be hand-edited unless you know what you're doing.
- The state file is rewritten atomically (write + rename), so a crash
  during a flush will not corrupt prior state.
