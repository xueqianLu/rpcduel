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
| `--input` | | JSON batch-request file `[{method, params}]` |
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

## See also

* [`replay`](/data-driven/replay) — run thousands of diffs from real on-chain data
* [SLO thresholds](/advanced/thresholds) for failing CI on diff rate
