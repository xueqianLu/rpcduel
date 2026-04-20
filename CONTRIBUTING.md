# Contributing to rpcduel

Thanks for your interest in improving **rpcduel**! This document explains how
to set up a development environment, the conventions we follow, and how to
submit a change.

## Prerequisites

- Go **1.23 or later** (CI tests against 1.23 and 1.24)
- `make` (optional, but recommended)
- `golangci-lint` v1.61+ for local linting
- `goreleaser` v2+ if you want to test release packaging locally

## Building and testing

```bash
make build    # build ./bin/rpcduel
make test     # go test ./...
make race     # go test -race ./...
make cover    # write coverage.out + print summary
make vet      # go vet ./...
make lint     # golangci-lint run
```

Or run the underlying Go commands directly — the Makefile is just convenience.

## Project layout

```
cmd/         Cobra subcommands (one file per command)
internal/    Implementation packages, not importable from outside this module
  rpc/      JSON-RPC HTTP client
  diff/     Deep JSON comparison
  bench/    Latency / QPS metrics
  dataset/  On-chain data collection
  replay/   Data-driven consistency testing
  benchgen/ Scenario generation + load test
  report/   Output formatting
  runner/   Concurrent worker pool
```

When adding a new subcommand:
1. Add `cmd/<name>.go` with a `cobra.Command` and an `init()` that wires flags.
2. Register it from `cmd/root.go`.
3. Put the heavy lifting in a focused package under `internal/`.
4. Add unit tests next to the implementation (`*_test.go`).
5. Update `README.md` with flags and an example.

## Coding conventions

- Format with `gofmt` / `goimports` (CI enforces this).
- Prefer small, testable packages over large ones.
- Public APIs in `internal/` should have doc comments.
- For RPC interactions, always pass a `context.Context` and respect timeouts.
- Errors should be wrapped with `fmt.Errorf("...: %w", err)` so callers can
  inspect them.

## Commit messages

We loosely follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(replay): add --only flag for selective targets
fix(diff): normalise hex addresses to lowercase before comparing
docs: clarify dataset workflow
chore: bump go to 1.23
```

This makes the auto-generated changelog readable.

## Pull requests

1. Fork and create a feature branch from `main`.
2. Add tests that exercise the new behavior.
3. Run `make test lint` locally and make sure CI is green.
4. Keep the PR focused — split unrelated changes into separate PRs.
5. Update `CHANGELOG.md` under the `Unreleased` section.

## Reporting issues

Please use the issue templates and include:
- `rpcduel` version (`rpcduel --version`) or commit hash
- Go version and OS
- The exact command you ran and the output you observed
- What you expected to happen instead

## Releasing (maintainers)

1. Update `CHANGELOG.md` and move the `Unreleased` items under a new version.
2. Tag the commit: `git tag -s vX.Y.Z -m "vX.Y.Z" && git push --tags`.
3. The `release` workflow will run GoReleaser and publish:
   - GitHub Release with cross-platform archives + checksums
   - Multi-arch container image at `ghcr.io/xueqianlu/rpcduel`
