# `diff`

Compare the response of any JSON-RPC method across two or more endpoints.

```
rpcduel diff [flags]
```

The diff engine performs a deep JSON comparison with **hex/decimal normalisation** (so `"0x1a"` and
`"26"` are equal), supports per-field ignoring, and can treat arrays as unordered sets.

## Flags

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required, ≥2)_ | Endpoint URLs to compare |
| `--method` | `eth_blockNumber` | JSON-RPC method |
| `--params` | `[]` | JSON-encoded params array |
| `--input` | | Requests file: JSON array, single object, or NDJSON (one JSON-RPC request per line). Extra fields like `jsonrpc`/`id` are ignored. |
| `--repeat` | `1` | Repeat each request N times |
| `--ignore-field` | | Field name(s) to skip in comparison |
| `--ignore-order` | `false` | Treat arrays as unordered sets |
| `--timeout` | `30s` | Per-request timeout |
| `--output` | `text` | `text` or `json` |

## Examples

```bash
# Compare eth_blockNumber across two nodes
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com

# Compare a specific block, ignoring the logsBloom field
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --method eth_getBlockByNumber \
  --params '["0x1000000", false]' \
  --ignore-field logsBloom

# Load a batch of requests from a file and output JSON
rpcduel diff \
  --rpc https://rpc-a.example.com \
  --rpc https://rpc-b.example.com \
  --input requests.json \
  --output json
```

`requests.json`:

```json
[
  {
    "method": "eth_getBalance",
    "params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]
  },
  {
    "method": "eth_getTransactionCount",
    "params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "latest"]
  }
]
```

You can also feed an NDJSON file (one full JSON-RPC request per line, e.g.
captured from a node's access log). Extra fields like `jsonrpc` and `id`
are ignored, and lines starting with `#` are treated as comments:

```text
# captured from access log on 2026-05-10
{"jsonrpc":"2.0","id":2076,"method":"eth_call","params":[{"data":"0x06fdde03","to":"0x0000000000000000000000000000000000000100"},"0xa0"]}
{"jsonrpc":"2.0","id":2077,"method":"eth_call","params":[{"data":"0x95d89b41","to":"0x0000000000000000000000000000000000000100"},"latest"]}
```

```bash
rpcduel diff \
  --rpc http://node-a:8545 \
  --rpc http://node-b:8545 \
  --input captured.ndjson \
  --output json
```

## See also

* [`replay`](/data-driven/replay) — run thousands of diffs from real on-chain data
* [SLO thresholds](/advanced/thresholds) for failing CI on diff rate
