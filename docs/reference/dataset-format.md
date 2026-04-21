# Dataset File Format

The JSON file produced by [`rpcduel dataset`](/data-driven/dataset) and consumed by
[`replay`](/data-driven/replay) and [`benchgen`](/data-driven/benchgen).

## Schema

```json
{
  "meta": {
    "chain": "ethereum",
    "rpc": "https://rpc.example.com",
    "generated_at": "2026-03-23T06:00:00Z"
  },
  "range": { "from": 20000000, "to": 20001000 },
  "accounts": [
    {
      "address": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
      "tx_count": 1234,
      "transactions": [
        {
          "hash": "0xabc123…",
          "block_number": 20000500,
          "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
          "to": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
        }
      ]
    }
  ],
  "transactions": [
    {
      "hash": "0xabc123…",
      "block_number": 20000500,
      "from": "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
      "to": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
    }
  ],
  "blocks": [
    { "number": 20001000, "tx_count": 142 },
    { "number": 20000500, "tx_count": 87 }
  ]
}
```

## Sort order (deterministic)

| Section | Order |
|---|---|
| `accounts` | `tx_count` descending (most active first) |
| `blocks` | `number` descending (newest first) |
| `transactions` | `block_number` ascending (chronological) |

## Notes

* Each account record carries its own `transactions` list (up to `--max-tx-per-account` entries) so [`replay`](/data-driven/replay) can query historical state at the right block heights without re-fetching tx data.
* The file is plain JSON — use `jq` / your editor to inspect or hand-edit it.
* The same dataset is reusable across many runs; re-generate only when you want to refresh the chain window.
