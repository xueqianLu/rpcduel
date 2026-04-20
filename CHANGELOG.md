# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
