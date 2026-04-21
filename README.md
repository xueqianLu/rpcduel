# rpcduel

[![CI](https://github.com/xueqianLu/rpcduel/actions/workflows/ci.yml/badge.svg)](https://github.com/xueqianLu/rpcduel/actions/workflows/ci.yml)
[![Release](https://github.com/xueqianLu/rpcduel/actions/workflows/release.yml/badge.svg)](https://github.com/xueqianLu/rpcduel/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/xueqianLu/rpcduel.svg)](https://pkg.go.dev/github.com/xueqianLu/rpcduel)
[![Go Report Card](https://goreportcard.com/badge/github.com/xueqianLu/rpcduel)](https://goreportcard.com/report/github.com/xueqianLu/rpcduel)
[![License: Apache 2.0](https://img.shields.io/github/license/xueqianLu/rpcduel)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/xueqianLu/rpcduel?sort=semver)](https://github.com/xueqianLu/rpcduel/releases)

**rpcduel** is a high-performance CLI for comparing and benchmarking
Ethereum JSON-RPC endpoints. A single binary lets you call any RPC,
diff responses across nodes, run concurrent benchmarks, and — its
signature feature — drive realistic consistency tests and load tests
from on-chain data you collect yourself.

> 📖 **Full documentation: <https://xueqianlu.github.io/rpcduel/>**

---

## ✨ What makes rpcduel different

The **`dataset → replay / benchgen`** pipeline is the headline feature.
You scan a real block range once, then reuse that dataset to run
**data-driven consistency tests** (`replay`) and **realistic
mixed-traffic load tests** (`benchgen`) against any pair of nodes — no
scripting, no manual fixtures.

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

Or skip the two steps entirely with `rpcduel record`, which captures
straight into a runnable `bench.json` scenario file.

---

## Quick start

### Install

Download a prebuilt binary from the
[latest release](https://github.com/xueqianLu/rpcduel/releases), or:

```bash
# Docker
docker run --rm ghcr.io/xueqianlu/rpcduel:latest call --rpc https://rpc.example.com

# Go install (requires Go 1.23+)
go install github.com/xueqianLu/rpcduel@latest
```

See [Installation](https://xueqianlu.github.io/rpcduel/guide/installation)
for Homebrew, package managers, and shell completions.

### A minute of rpcduel

```bash
# 1. Sanity-check a couple of RPCs.
rpcduel doctor --rpc https://rpc.a --rpc https://rpc.b

# 2. Capture real on-chain traffic into a bench scenario.
rpcduel record --rpc https://rpc.a --max-blocks 200 --out bench.json

# 3. Compare correctness of two nodes against a recorded dataset.
rpcduel dataset --rpc https://rpc.a --out dataset.json
rpcduel replay --dataset dataset.json --rpc https://rpc.a --rpc https://rpc.b

# 4. Load-test both nodes with realistic mixed traffic.
rpcduel bench --input bench.json --rpc https://rpc.a --rpc https://rpc.b --duration 30s
```

---

## Commands at a glance

### Basics

| Command | Purpose |
|---|---|
| [`call`](https://xueqianlu.github.io/rpcduel/commands/call) | Call any JSON-RPC method against one endpoint |
| [`diff`](https://xueqianlu.github.io/rpcduel/commands/diff) | Diff a single RPC call across two endpoints |
| [`bench`](https://xueqianlu.github.io/rpcduel/commands/bench) | Concurrent benchmark with QPS / P95 / P99 / HDR |
| [`duel`](https://xueqianlu.github.io/rpcduel/commands/duel) | Side-by-side benchmark of two endpoints |

### Data-driven workflow ⭐

| Command | Purpose |
|---|---|
| [`dataset`](https://xueqianlu.github.io/rpcduel/data-driven/dataset) | Scan a chain and capture accounts / txs / blocks |
| [`dataset inspect`](https://xueqianlu.github.io/rpcduel/data-driven/dataset#inspect-a-dataset-file) | Summarize a dataset file (counts, top accounts, estimated load) |
| [`replay`](https://xueqianlu.github.io/rpcduel/data-driven/replay) | Replay every dataset call across two endpoints and diff results |
| [`benchgen`](https://xueqianlu.github.io/rpcduel/data-driven/benchgen) | Turn a dataset into a weighted bench scenario file |
| [`record`](https://xueqianlu.github.io/rpcduel/data-driven/record) | One-shot dataset → bench scenario capture |

Long-running `replay` and `bench` jobs support `--state-file` /
`--resume` checkpointing — see
[Resuming long runs](https://xueqianlu.github.io/rpcduel/data-driven/resume).

### Advanced

| Topic | Doc |
|---|---|
| Configuration file (`rpcduel.yaml`) | [advanced/config](https://xueqianlu.github.io/rpcduel/advanced/config) |
| SLO thresholds & CI gating | [advanced/thresholds](https://xueqianlu.github.io/rpcduel/advanced/thresholds) |
| HTML / Markdown / JUnit reports | [advanced/reports](https://xueqianlu.github.io/rpcduel/advanced/reports) |
| Prometheus + Pushgateway | [advanced/metrics](https://xueqianlu.github.io/rpcduel/advanced/metrics) |
| `doctor` capability probe | [advanced/doctor](https://xueqianlu.github.io/rpcduel/advanced/doctor) |
| CI templates (GitHub Actions) | [advanced/ci](https://xueqianlu.github.io/rpcduel/advanced/ci) |
| Shell completions & man pages | [advanced/completions](https://xueqianlu.github.io/rpcduel/advanced/completions) |

---

## Project status

Early but actively used. See [CHANGELOG.md](./CHANGELOG.md) for
release notes and [issues](https://github.com/xueqianLu/rpcduel/issues)
for what's planned.

## Contributing

Contributions welcome — please read [CONTRIBUTING.md](./CONTRIBUTING.md)
and our [security policy](./SECURITY.md). Bug reports, feature
requests, and PRs are all appreciated.

## License

Apache License 2.0 — see [LICENSE](./LICENSE) and [AUTHORS](./AUTHORS).
