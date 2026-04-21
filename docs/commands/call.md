# `call`

Call any JSON-RPC method directly against a single endpoint. The fastest way to replace one-off
`curl` commands when debugging or exploring node behavior.

```
rpcduel call [method] [param...] [flags]
```

When a method and params are provided positionally, `rpcduel` uses them directly:

```bash
rpcduel call --rpc https://rpc.example.com eth_getBalance 0xa11111 latest
```

## Positional parsing

Positional params are parsed smartly:

- plain tokens like `latest`, `0xa11111`, addresses, and tx hashes stay as strings
- JSON literals like `true`, `false`, `null`, `123`, `{"k":1}`, and `[1,2]` are decoded as JSON

To avoid ambiguity, positional params **cannot** be mixed with `--params` or `--params-file`.

## Flags

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required)_ | Endpoint URL to call |
| `--method` | `eth_blockNumber` | JSON-RPC method |
| `--params` | `[]` | JSON-encoded params array |
| `--params-file` | | JSON file containing the params array; overrides `--params` |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

## Examples

```bash
# Quick block number query
rpcduel call --rpc https://rpc.example.com

# Preferred shorthand: method + params as positional arguments
rpcduel call \
  --rpc https://rpc.example.com \
  eth_getBalance 0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045 latest

# Flag-based form is still supported
rpcduel call \
  --rpc https://rpc.example.com \
  --method eth_getBalance \
  --params '["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]'

# Load params from a file to avoid shell escaping
rpcduel call \
  --rpc https://rpc.example.com \
  --method eth_getLogs \
  --params-file params.json \
  --output json
```

## See also

* [`diff`](/commands/diff) — compare a method across multiple endpoints
* [Output formats](/reference/output-formats)
