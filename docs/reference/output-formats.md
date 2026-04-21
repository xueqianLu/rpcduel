# Output Formats

All commands support `--output text` (default) and `--output json`. Replay additionally supports
the [report flags](/advanced/reports) for HTML / Markdown / CSV / JUnit.

## `call`

### Text

```
RPC Call Result
----------------------------------------
Endpoint: https://rpc.example.com
Method:   eth_getBalance
Latency:  12.345ms
Params:
  [
    "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045",
    "latest"
  ]
Result:
  "0x1a2b3c"
```

### JSON

```json
{
  "endpoint": "https://rpc.example.com",
  "method": "eth_blockNumber",
  "params": [],
  "success": true,
  "latency_ms": 4.218,
  "result": "0x1312d00"
}
```

## `bench`

### Text

```
Benchmark Result
----------------------------------------
Endpoint:   https://rpc.example.com
  Requests: 10000
  Errors:   12 (0.1%)
  QPS:      843.21
  Avg:      11.856ms
  P95:      24.301ms
  P99:      38.442ms
```

## `replay`

### Text

```
Replay Result
----------------------------------------
Accounts tested:     500
Transactions tested: 500
Blocks tested:       200
Total requests:      1700
Success rate:        99.8%
Unsupported:         2
Total diffs:         3

Diff summary:
  - balance_mismatch: 2
  - missing_data: 1

Sample diffs (up to 10):
  [balance_mismatch] eth_getBalance: [$.] 0x1a2b3c vs 0x1a2b3d (value mismatch)
```

### JSON

```json
{
  "accounts_tested": 500,
  "transactions_tested": 500,
  "blocks_tested": 200,
  "total_requests": 1700,
  "success_rate": 0.998,
  "unsupported": 2,
  "total_diffs": 3,
  "diff_summary": {
    "balance_mismatch": 2,
    "missing_data": 1
  },
  "sample_diffs": [
    {
      "Category": "balance_mismatch",
      "Method": "eth_getBalance",
      "Params": ["0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045", "0x1312d00"],
      "Detail": "[$.] 0x1a2b3c vs 0x1a2b3d (value mismatch)"
    }
  ]
}
```
