# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- GitHub Actions CI workflow running `go vet`, `go build`, `go test -race`, and
  `golangci-lint` across Linux/macOS/Windows on Go 1.23 and 1.24.
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

### Changed
- Lowered required Go version from 1.24.13 to **1.23** for broader
  compatibility.
- README: added build/test/release badges and updated install instructions to
  cover prebuilt binaries and Docker images.

### Removed
- Unused Blockscout REST API client (`internal/dataset/blockscout.go` and
  its tests). The `dataset` command has scanned chain data via JSON-RPC since
  the previous release; this code was no longer wired in.
