# Configuration File

Pass `--config rpcduel.yaml` (or `-c rpcduel.yaml`) to load defaults for any subcommand.

> CLI flags **always** override config values. Environment variables expand as `${VAR}` or
> `${VAR:-default}`. Use `$$` for a literal `$`.

A complete annotated sample lives at [`examples/rpcduel.yaml`](https://github.com/xueqianLu/rpcduel/blob/main/examples/rpcduel.yaml).

## Example

```yaml
version: 1

endpoints:
  - name: geth
    url: http://localhost:8545
    headers:
      X-Source: rpcduel
  - name: erigon
    url: ${ERIGON_URL:-http://localhost:8546}
    headers:
      Authorization: Bearer ${ERIGON_TOKEN}

defaults:
  log_level: info
  log_format: text
  retries: 2
  insecure: false
  user_agent: rpcduel/0.2.0
  headers:
    X-Run-Id: ${GITHUB_RUN_ID:-local}

bench:
  method: eth_blockNumber
  concurrency: 16
  duration: 30s
  timeout: 10s
  warmup: 5s

duel:
  method: eth_getBlockByNumber
  params: '["latest", false]'
  concurrency: 8
  duration: 30s
  ignore_fields: [hash, miner]

diff:
  method: eth_getLogs
  params: '[{"fromBlock":"latest"}]'
  repeat: 5

replay:
  dataset: dataset.json
  max_tx_per_account: 5
  ignore_fields: [hash]
  concurrency: 4
  report: replay.txt
  csv: replay.csv

thresholds:
  bench:
    p95_ms: 250
    p99_ms: 500
    error_rate: 0.01
  duel:
    p99_ms: 500
    error_rate: 0.01
    diff_rate: 0.001
  diff:
    diff_rate: 0.0
    max_diffs: 0
  replay:
    mismatch_rate: 0.001
    error_rate: 0.01
    max_mismatch: 5

reports:
  html: report.html
  markdown: report.md
  junit: report.junit.xml
```

## Sections

| Section | Purpose |
|---|---|
| `version` | Schema version (currently `1`) |
| `endpoints` | Named endpoint catalogue (referenced from CLI by name) |
| `defaults` | Global flag defaults |
| `bench` / `duel` / `diff` / `replay` | Per-command flag defaults |
| `thresholds` | [SLO thresholds](/advanced/thresholds) — non-zero exit on breach |
| `reports` | Default report file destinations (CLI overrides per-flag) |

## See also

* [SLO thresholds](/advanced/thresholds)
* [Reports](/advanced/reports)
* [Global flags](/guide/global-flags)
