# `dataset`

Scan a block range **from high to low** via an Ethereum JSON-RPC endpoint and save a
representative set of blocks, transactions, and accounts to a JSON file for use with
[`replay`](/data-driven/replay) and [`benchgen`](/data-driven/benchgen).

The scanner calls `eth_getBlockByNumber` (with full transaction objects) for every block in the
range, collects non-empty blocks, extracts transactions, and ranks all addresses by activity — no
external explorer required.

```
rpcduel dataset [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--rpc` | _(required)_ | Ethereum JSON-RPC endpoint URL |
| `--from-block` | `0` | Start block, inclusive (0 = `toBlock − blocks×10`) |
| `--to-block` | `0` | End block, inclusive (0 = current chain head) |
| `--accounts` | `1000` | Max accounts to collect (sorted by observed tx count) |
| `--txs` | `1000` | Max transactions to collect |
| `--blocks` | `1000` | Max non-empty blocks to collect |
| `--max-tx-per-account` | `100` | Max transactions stored per account (0 = unlimited) |
| `--concurrency` | `4` | Number of goroutines used to fetch blocks |
| `--chain` | `ethereum` | Chain name written to dataset metadata |
| `--out` | `dataset.json` | Output file path |

## Example

```bash
rpcduel dataset \
  --rpc https://rpc.example.com \
  --from-block 20000000 \
  --to-block 20001000 \
  --accounts 500 --txs 500 --blocks 200 \
  --chain ethereum \
  --out mainnet-dataset.json
```

## Behaviour notes

* Scanning stops early once **all three** collection limits (`--accounts`, `--txs`, `--blocks`) are satisfied.
* When `--to-block` is omitted, the current chain head is resolved automatically via `eth_blockNumber`.
* Per-account transaction lists (up to `--max-tx-per-account`) are embedded directly in each account record. This lets [`replay`](/data-driven/replay) query historical state at the correct block numbers without re-fetching transaction lists at test time.
* Output is **deterministically ordered**: accounts by tx count (desc), blocks by number (desc), transactions by block number (asc).
* Network errors and timeouts are logged as warnings; already-collected data is saved regardless.

## Inspect a dataset file

Once you have a `dataset.json`, use `dataset inspect` to see what's in
it before piping to `replay` or `benchgen`:

```bash
rpcduel dataset inspect dataset.json
rpcduel dataset inspect dataset.json --json
```

The text output includes:

- meta (chain, source RPC, generation timestamp, schema version)
- block range and counts (accounts / unique tx hashes / blocks)
- top-10 accounts by transaction count
- a tx-per-account distribution histogram
- estimated RPC call counts per replay/benchgen category, useful for
  budgeting how long a downstream `replay` or `bench` will take

`--json` emits the same data as a stable JSON document — safe to feed
to `jq` or assert against in CI.

## See also

* [Dataset file format](/reference/dataset-format)
* [`replay`](/data-driven/replay) — consistency test from this dataset
* [`benchgen`](/data-driven/benchgen) — load test from this dataset
