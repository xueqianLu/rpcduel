# Why rpcduel?

**rpcduel** is a high-performance CLI for comparing and benchmarking Ethereum JSON-RPC endpoints.
A single binary lets you call any RPC, diff responses across nodes, run concurrent benchmarks,
and — its signature feature — drive realistic consistency tests and load tests from on-chain data
**you collect yourself**.

## When to reach for it

| You want to… | Use |
|---|---|
| Replace one-off `curl` calls when debugging a node | [`call`](/commands/call) |
| Verify two nodes agree on a method's response | [`diff`](/commands/diff) |
| Stress a single endpoint with concurrent traffic | [`bench`](/commands/bench) |
| Run diff and bench against two endpoints in one go | [`duel`](/commands/duel) |
| **Run a full data-driven consistency test from real blocks** | [`replay`](/data-driven/replay) ⭐ |
| **Benchmark with realistic mixed traffic from real blocks** | [`benchgen`](/data-driven/benchgen) ⭐ |
| Pre-flight a node's RPC capabilities in CI | [`doctor`](/advanced/doctor) |

## What sets it apart

> The **`dataset → replay / benchgen`** workflow is what makes rpcduel different from generic
> benchmarking tools. Instead of asking you to invent fixtures or write traffic generators,
> rpcduel scans real blocks once, persists them to a `dataset.json`, and lets you replay that
> data as either a consistency test or a load test.

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

Continue to the [data-driven workflow](/data-driven/workflow) to see the full pipeline,
or [install rpcduel](/guide/installation) and try the [basic commands](/commands/call) first.

## Designed for CI

* Non-zero exit codes for SLO breaches
* JUnit XML reports (consumable by GitHub Actions / GitLab test reporters)
* HTML & Markdown reports for human review
* Prometheus exporter + Pushgateway for short-lived runs
* `doctor` pre-flight to fail fast when a node is unreachable
* Drop-in [GitHub Actions / GitLab CI templates](/advanced/ci)
