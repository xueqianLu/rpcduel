# `rpcduel contract`

Read-only convenience helpers for inspecting well-known standard contracts
(ERC-20, ERC-721) and generic on-chain state. Every subcommand is a thin
wrapper around `eth_call` / `eth_getStorageAt` / `eth_getCode`, so it works
against any standards-compliant Ethereum JSON-RPC endpoint.

For arbitrary read-only contract calls (custom ABIs, non-standard methods),
use [`rpcduel call`](./call.md) with `eth_call` directly.

## Common flags

| Flag | Default | Description |
|------|---------|-------------|
| `--rpc URL` | _(required)_ | RPC endpoint URL. |
| `--block` | `latest` | Block tag (`latest`, `pending`, `finalized`, `safe`) or `0x`-hex / decimal block number. |
| `--output` | `text` | `text` or `json`. |
| `--timeout` | `15s` | Per-request timeout. |

## ERC-20

### `contract erc20 info <token>`

Print `name`, `symbol`, `decimals`, and `totalSupply`. Falls back to the
legacy `bytes32` encoding for tokens like MKR/SAI.

```bash
rpcduel contract erc20 info \
  --rpc https://eth.llamarpc.com \
  0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48     # USDC
```

### `contract erc20 balance <token> <holder>`

Print the holder's balance, formatted with the token's decimals.

```bash
rpcduel contract erc20 balance \
  --rpc https://eth.llamarpc.com \
  0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48 \
  0x28C6c06298d514Db089934071355E5743bf21d60
```

### `contract erc20 allowance <token> <owner> <spender>`

Print `allowance(owner, spender)` formatted with the token's decimals.

## ERC-721

### `contract erc721 info <token>`

Print `name`, `symbol`, and (if the contract implements ERC-721 Enumerable)
`totalSupply`. A revert on `totalSupply()` is reported under `errors` but is
expected for non-Enumerable collections.

### `contract erc721 owner <token> <token-id>`

Print `ownerOf(token-id)`. `<token-id>` accepts decimal or `0x`-hex.

### `contract erc721 tokenURI <token> <token-id>`

Print `tokenURI(token-id)`.

## Generic helpers

### `contract storage <address> <slot>`

Read the 32-byte storage word at `slot` of `address` (`eth_getStorageAt`).
Slot accepts decimal or `0x`-hex.

```bash
rpcduel contract storage \
  --rpc https://eth.llamarpc.com \
  0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48 0x0
```

### `contract code <address>`

Fetch deployed bytecode (`eth_getCode`). Prints size and the first 64 bytes
by default; pass `--full` to dump the full bytecode hex. An EOA prints a
clear `(none — externally owned account…)` message.

```bash
rpcduel contract code \
  --rpc https://eth.llamarpc.com \
  --full \
  0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
```

## JSON output

All subcommands accept `--output json` for piping into `jq` or downstream
tooling. Example:

```bash
rpcduel contract erc20 info \
  --rpc $RPC --output json \
  0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48 | jq .symbol
```
