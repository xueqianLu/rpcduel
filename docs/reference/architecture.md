# Architecture

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

## Key design decisions

* **Hex normalisation** — `"0x1a"` and `"26"` are equal in the diff engine, avoiding false
  positives from encoding differences.
* **Weighted dispatch** — `benchgen` assigns a weight to every scenario; sampling is proportional,
  so realistic mixed traffic emerges without manual scripting.
* **Archive-node detection** — `replay` recognises `missing trie node` / `state not found` errors
  and marks those requests as `unsupported` rather than counting them as mismatches.
* **Graceful partial results** — network errors and API timeouts during dataset collection are
  logged as warnings; already-collected data is saved regardless.
* **CI-first** — non-zero exit codes for SLO breaches, JUnit reports, doctor pre-flight, and
  Pushgateway support so rpcduel slots straight into existing pipelines.

## Module boundaries

| Package | Imports? | Notes |
|---|---|---|
| `internal/rpc` | std + minimal | The transport core. No knowledge of diff/bench/replay. |
| `internal/diff` | std | Pure value comparison. No I/O. |
| `internal/bench` | `internal/rpc`, `internal/runner` | Latency/QPS aggregation only. |
| `internal/replay` | `internal/rpc`, `internal/diff`, `internal/dataset` | Iterates a dataset, classifies diffs. |
| `internal/benchgen` | `internal/dataset` | Pure scenario synthesis; runtime delegates to `internal/runner`. |
| `internal/report` | std | Renders results to many formats. No I/O of its own beyond the writer it's given. |
| `internal/metrics` | `prometheus/client_golang` | Optional; the rest of the binary works without it. |
