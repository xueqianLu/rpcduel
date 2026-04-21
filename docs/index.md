---
layout: home

hero:
  name: rpcduel
  text: Compare. Diff. Benchmark. Replay.
  tagline: A high-performance CLI for Ethereum JSON-RPC endpoints — drive realistic consistency tests and load tests from real on-chain data.
  image:
    src: /logo.svg
    alt: rpcduel
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: Data-Driven Workflow ⭐
      link: /data-driven/workflow
    - theme: alt
      text: View on GitHub
      link: https://github.com/xueqianLu/rpcduel

features:
  - icon: 🛰️
    title: Direct RPC calls
    details: Invoke any JSON-RPC method from the CLI without hand-writing curl. HTTP(S), WebSocket and Unix IPC endpoints all supported transparently.
    link: /commands/call
    linkText: rpcduel call

  - icon: 🪞
    title: Response diffing
    details: Deep JSON comparison with hex/decimal normalisation, field ignoring, and order-insensitive arrays.
    link: /commands/diff
    linkText: rpcduel diff

  - icon: 🚀
    title: Concurrent benchmarks
    details: HDR-histogram backed P50/P95/P99/P999 latency with token-bucket rate limiting and warm-up phase.
    link: /commands/bench
    linkText: rpcduel bench

  - icon: ⭐
    title: Data-driven testing
    details: Scan real blocks once, then reuse the dataset to run consistency tests (replay) and realistic mixed-traffic load (benchgen) — no scripting required.
    link: /data-driven/workflow
    linkText: dataset → replay / benchgen

  - icon: ✅
    title: CI-first
    details: SLO thresholds with non-zero exit, JUnit / HTML / Markdown reports, Prometheus exporter and Pushgateway, plus doctor pre-flight.
    link: /advanced/thresholds
    linkText: SLO thresholds

  - icon: 📦
    title: Single static binary
    details: Pre-built binaries for Linux, macOS and Windows (amd64/arm64). Docker image, Homebrew tap and shell completions out of the box.
    link: /guide/installation
    linkText: Install
---

<div style="max-width: 960px; margin: 4rem auto 0; padding: 0 24px;">

## Why rpcduel?

The **`dataset → replay / benchgen`** workflow is what sets rpcduel apart from generic benchmarking tools.
You scan a real block range once, then reuse that dataset to run **data-driven consistency tests** and
**realistic mixed-traffic load tests** against any pair of nodes — no scripting, no manual fixtures.

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

[Jump to the workflow →](/data-driven/workflow)

</div>
