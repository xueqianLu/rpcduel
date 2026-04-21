# `doctor` — Endpoint Capability Check

A fast pre-flight that probes a curated set of methods against every endpoint in parallel and
prints a capability matrix. Use it as a **hard gate at the top of CI pipelines** so unreachable or
mis-configured nodes fail loudly before you spend budget on a full replay run.

```
rpcduel doctor [flags]
```

## Default probes

* `web3_clientVersion`
* `eth_chainId`
* `eth_blockNumber`
* `eth_syncing`
* `net_peerCount`
* `eth_gasPrice`

Plus anything you pass via `--probe` (repeatable).

## Flags

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required, ≥1)_ | Endpoint URL(s) to probe |
| `--probe` | (none) | Extra JSON-RPC methods to probe, repeatable |
| `--timeout` | `10s` | Per-probe timeout |
| `--output` | `text` | `text` or `json` |
| `--fail-on` | `unreachable` | `never`, `unreachable`, or `any`. Failure exits with code **2**. |

## `--fail-on` semantics

| Value | Exits 2 when… |
|---|---|
| `never` | Never. Always exit 0 (informational only). |
| `unreachable` _(default)_ | An endpoint cannot complete `web3_clientVersion` / `eth_chainId` / `eth_blockNumber`. |
| `any` | Any single probe (default or `--probe`) returns an error for any endpoint. |

## Examples

```bash
# Quick check, exit 2 if either node is unreachable
rpcduel doctor \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com

# Probe trace methods too and fail if either node lacks them
rpcduel doctor \
  --rpc https://node-a.example.com \
  --rpc https://node-b.example.com \
  --probe debug_traceTransaction \
  --probe debug_traceBlockByNumber \
  --fail-on any

# JSON output, useful for further pipeline steps
rpcduel doctor --output json --rpc $NODE_A --rpc $NODE_B
```

## See also

* [SLO thresholds](/advanced/thresholds) — also exit code 2
* [CI templates](/advanced/ci) — `doctor` is wired in as the first step
