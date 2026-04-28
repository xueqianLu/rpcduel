# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `--tracer` / `--tracer-config` flags for `rpcduel benchgen`, `rpcduel
  replay`, and `rpcduel record`. The default tracer for the
  `debug_trace*` family is now `callTracer` (the de-facto standard used
  by indexers, explorers and audit tooling) instead of the node's
  built-in `structLogger`. Pass `--tracer default` to restore the
  pre-flag behaviour. `--tracer-config` accepts a raw JSON object,
  e.g. `--tracer prestateTracer --tracer-config '{"diffMode":true}'`
  for state-diff mode.
- `rpcduel contract` — new top-level command grouping read-only helpers
  for well-known standard contracts:
  - `contract erc20 {info|balance|allowance}` (with `bytes32` fallback
    for legacy tokens like MKR/SAI).
  - `contract erc721 {info|owner|tokenURI}` (Enumerable `totalSupply`
    is reported as a soft error when the contract does not implement
    it).
  - `contract storage <address> <slot>` and `contract code <address>`
    for generic on-chain inspection.
  - All subcommands accept `--rpc`, `--block`, `--output text|json`,
    and `--timeout`. For arbitrary `eth_call` invocations continue to
    use `rpcduel call`.
- `rpcduel record` — one-shot capture from a live endpoint into a
  bench scenario file, bridging `dataset` → `benchgen` into a single
  command. Supports `--methods` whitelist and per-scenario `--sample`
  fraction so the resulting `bench.json` is small enough to commit.
- `rpcduel dataset inspect <file>` — print a summary of a captured
  dataset (counts, block range, top-10 accounts, tx-per-account
  buckets, estimated request counts per replay/benchgen category).
  `--json` for machine-readable output.
- `--state-file` / `--resume` for `rpcduel replay` — periodic
  checkpoint of completed task keys + counters + diffs (every
  `--state-interval` tasks and on Ctrl+C). Resuming skips
  already-completed task keys via the canonical `(method, params)`
  hash, so interrupted long-running replays can be safely continued.
- `--state-file` / `--resume` for `rpcduel bench` (single-method
  `--requests N` mode only). Persists per-endpoint HDR snapshot,
  totals, errors, and start time. Resume continues with the remaining
  request count and merges into the existing histogram.

## [0.2.0] - 2026-04-21

### Added
- `rpcduel doctor` — connectivity, identity, and capability probe. For
  every `--rpc` endpoint it runs `web3_clientVersion`, `eth_chainId`,
  `eth_blockNumber`, `eth_syncing`, `net_peerCount`, and `eth_gasPrice`
  in parallel, plus any extra methods supplied via `--probe`. Output is
  a compact text table (default) or full JSON. Gating is controlled by
  `--fail-on {never|unreachable|any}`; on failure the command exits 2
  for CI use. Great as the first step of any rpcduel pipeline.
- Prometheus Pushgateway exporter. New global flags
  `--push-gateway URL`, `--push-job NAME`, and repeatable
  `--push-label key=value` cause rpcduel to push every metric in its
  registry (requests total, latency histogram, diff counters, **new
  replay category counter**) to a Prometheus Pushgateway at command
  exit. Complements the existing `--metrics-addr` pull-mode endpoint.
- Replay diff category histogram. `replay`'s category breakdown
  (`balance_mismatch`, `nonce_mismatch`, `tx_mismatch`,
  `receipt_mismatch`, `trace_mismatch`, `block_mismatch`,
  `missing_data`, `rpc_error`) is now surfaced three ways: the new
  `rpcduel_replay_diffs_total{category=…}` Prometheus counter, a
  deterministic bar chart in the HTML and Markdown reports, and one
  JUnit `<testcase>` per category inside a dedicated
  `replay_categories` suite — so CI runners show the distribution
  without anyone opening the HTML.
- CI workflow templates in `examples/ci/` for GitHub Actions and
  GitLab CI, wiring `doctor`, `diff`, and `replay` with SLO thresholds
  and JUnit uploads. See `examples/ci/README.md`.

### Added (previously in Unreleased)
- YAML configuration file support. New global `--config / -c PATH` flag
  loads a `rpcduel.yaml` describing endpoints, per-section defaults
  (bench, duel, diff, replay), SLO thresholds, and report destinations.
  Environment variables expand inline via `${VAR}` and `${VAR:-default}`
  (literal `$$` escapes to `$`). CLI flags always override config
  values; config endpoints are used only when `--rpc` is not passed.
  Per-endpoint headers may be defined and merge over global
  `defaults.headers`. See `examples/rpcduel.yaml` for the full schema.
- SLO threshold gating for CI use. Each verb command accepts new
  shortcut flags (e.g. `--max-p99-ms`, `--max-error-rate`,
  `--max-diff-rate`, `--max-mismatch-rate`, `--max-mismatch`,
  `--max-diffs`) and reads the same limits from `thresholds:` in the
  config file. On any breach, rpcduel prints a `FAIL` summary to stderr
  listing each (endpoint, metric, limit, observed) tuple and exits with
  code 2 (distinct from the normal error code 1). When no thresholds
  are configured, behavior is unchanged.
- HTML, Markdown, and JUnit report exporters for `bench`, `duel`,
  `diff`, and `replay`. New per-command flags `--report-html PATH`,
  `--report-md PATH`, `--report-junit PATH` (also configurable via
  `reports.{html,markdown,junit}`) write self-contained reports
  alongside the existing stdout output.

## [0.1.0] - 2026-04-20

First public release.

### Added
- `dataset --append` for incremental dataset collection. When the
  destination file already exists, the scanner defaults the start block
  to `existingRange.To + 1`, fetches only the delta range, and merges
  the new accounts/transactions/blocks into the existing dataset
  (deduplicated by hash / address / number, with caps re-applied). The
  result preserves the union block range and keeps the largest reported
  per-account transaction count.
- Unix-domain-socket IPC transport. Endpoints with the `unix://` URL
  scheme (e.g. `unix:///tmp/geth.ipc`) connect directly to a node's IPC
  socket using the same multiplexed connection pattern as the WebSocket
  transport (single dial, id-keyed pending map, lazy reconnect).
- WebSocket transport for JSON-RPC endpoints. Any endpoint URL with a
  `ws://` or `wss://` scheme transparently uses a single multiplexed
  WebSocket connection (concurrent requests, response demultiplexed by
  JSON-RPC id) instead of HTTP. All existing flags (`--retries`,
  `--insecure`, `--header`, `--user-agent`) apply to the WS handshake.
- HDR histograms for latency tracking (1µs–60s, 3 sig figs) replacing the
  prior sort-based percentile calculation. New `p999_latency_ms` column in
  the bench text/CSV reports and `Summary.P999` field.
- `--rps` and `--rps-burst` global flags implementing a process-wide
  token-bucket rate limiter (via `golang.org/x/time/rate`) shared across all
  workers. Pair runs (`duel`, `replay`) take one token per logical pair.
- `--warmup <duration>` flag on `bench` and `duel` to discard results from
  an initial settling window so reported QPS/latency reflect only the
  steady-state measurement period.
- `--hdr-out <prefix>` flag on `bench` to dump per-endpoint HDR percentile
  logs (compatible with `hdr-plot`/`wrk2` tooling) as
  `<prefix>.<index>.hdr`.

### Fixed
- Runner entry points (`RunDuration`, `RunN`, `RunPaired`,
  `PairResultFromDuration`, `RunDurationFromTasks`,
  `RunDurationGenerated`) previously bypassed global flags `--retries`,
  `--insecure`, `--header`, and `--user-agent` by constructing
  `rpc.NewClient` directly. They now propagate the configured options via
  context, so all runners honor the documented HTTP behavior.

### Added
- GitHub Actions CI workflow running `go vet`, `go build`, `go test -race`, and
  `golangci-lint` across Linux and macOS on Go 1.23 and 1.24.
- `golangci-lint` configuration (`.golangci.yml`).
- GoReleaser configuration with cross-platform binaries (Linux/macOS/Windows ×
  amd64/arm64) and multi-arch container images published to
  `ghcr.io/xueqianlu/rpcduel`.
- Multi-stage `Dockerfile` (distroless runtime) for local builds and
  `Dockerfile.goreleaser` for release images.
- `Makefile` with `build`, `test`, `race`, `cover`, `vet`, `lint`, `tidy`,
  `release-snapshot`, and `docker` targets.
- `CONTRIBUTING.md` and GitHub issue / pull-request templates.
- `--version` flag printing version, commit, and build date (set via
  `-ldflags` at build time).
- Structured logging via `log/slog` with new global flags `--log-level` and
  `--log-format` (`text`/`json`).
- HTTP-level retries with exponential backoff for the JSON-RPC client.
  Configurable via `--retries` and `--retry-backoff`. Retries cover network
  errors, HTTP 408/429, and 5xx responses; JSON-RPC application errors are
  still surfaced immediately.
- Custom HTTP headers via repeatable `--header` flag (accepts both
  `Key: Value` and `Key=Value`) and `--user-agent` override.
- `examples/` directory with a getting-started README and a small batch
  request file.
- Dataset file format now embeds a `schema_version` field. Files written
  before this change (no version) are still accepted; files with a version
  newer than the running binary are rejected with a clear error.
- Dependabot configuration for Go modules, GitHub Actions, and Docker.
- CodeQL workflow with the `security-extended` query suite.
- `SECURITY.md` describing the vulnerability disclosure process.
- Release workflow gains a `validate` job that runs `goreleaser check` and
  a `workflow_dispatch` trigger that produces a snapshot build with
  artifacts uploaded for inspection (no publish, no tag required).
- All commands that accept `--output` now validate the value up front and
  return a clear error for unsupported formats (only `text` and `json`
  are valid).
- `make ci` convenience target runs `vet`, `lint`, and `race` together,
  mirroring the GitHub Actions CI job.
- `make manpages` and `make completions` regenerate man pages and shell
  completion scripts into `dist/man` and `dist/completions`. Both are bundled
  in every release archive.
- New global flag `--metrics-addr` exposes Prometheus metrics at `/metrics`
  for the duration of any `bench` or `duel` run. Counters track requests
  (`rpcduel_requests_total`) and diffs (`rpcduel_diffs_total`); a histogram
  tracks latency (`rpcduel_request_duration_seconds`).
- `Options.InsecureSkipVerify` (CLI: `--insecure`) to skip TLS certificate
  verification for development against self-signed nodes.
- `Options.Transport` lets callers inject a custom `http.RoundTripper`
  (mainly useful for tests).
- `.pre-commit-config.yaml` with `gofmt`, `go vet`, `go mod tidy`, and
  `golangci-lint` hooks.

### Changed
- Lowered required Go version from 1.24.13 to **1.23** for broader
  compatibility.
- README: added build/test/release badges, updated install instructions to
  cover prebuilt binaries and Docker images, and documented the new global
  flags.
- Replaced ad-hoc `fmt.Fprintf(os.Stderr, ...)` progress / warning lines in
  the `dataset`, `replay`, `duel`, `diff`, and `benchgen` commands with
  structured `slog` calls.
- CI matrix now covers only Linux and macOS (Windows dropped). The
  `golangci-lint` action is pinned to v1.64.5 (built with Go 1.24) so
  modern toolchains can lint locally without typecheck errors.

### Removed
- Unused Blockscout REST API client (`internal/dataset/blockscout.go` and
  its tests). The `dataset` command has scanned chain data via JSON-RPC since
  the previous release; this code was no longer wired in.
