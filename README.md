# rpcduel

`rpcduel` is a single-binary CLI for auditing and exercising Ethereum-style JSON-RPC endpoints.

## Commands

`dataset`

Scan a block range from one endpoint and stream unique block, transaction, and address records into `dataset.json`.

```bash
rpcduel dataset --to https://rpc.example --from 20000000 --to-block 20000010 --out dataset.json
```

`diff`

Compare block, transaction, receipt, balance, and nonce responses across multiple endpoints with semantic field masking.

```bash
rpcduel diff --to primary=https://rpc-a.example --to https://rpc-b.example --from 20000000 --to-block 20000001 --ignore size
```

If you want aliases instead of raw URLs, define them with repeated root-level `--provider alias=url` flags:

```bash
rpcduel --provider a=https://rpc-a.example --provider b=https://rpc-b.example diff --to a --to b --from 1 --to-block 2
```

`bench`

Replay a dataset through a concurrent worker pool and print RPS, P95/P99 latency, and error distribution.

```bash
rpcduel bench --to https://rpc.example --input dataset.json --concurrency 64 --requests 5000
```

`call`

Run a JSON-RPC method without hand-writing `curl` payloads. Results are pretty-printed, and quantity-like hex values are annotated with decimal equivalents.

```bash
rpcduel call eth_getBalance 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045 latest --to https://rpc.example
```

## Development

This scaffold uses:

- `spf13/cobra` and `spf13/pflag` for the CLI
- `pkg/rpc` for JSON-RPC transport, retries, and provider resolution
- `internal/diff` for recursive semantic diffing and block-range auditing

Core validation command:

```bash
GOROOT=/opt/homebrew/Cellar/go/1.24.0/libexec /opt/homebrew/bin/go test ./...
```
