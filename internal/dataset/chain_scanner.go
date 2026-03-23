// Package dataset – ChainScanner fetches chain data by iterating blocks
// via the Ethereum JSON-RPC API from a given high block down to a low block.
package dataset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// ChainScanner scans a block range via Ethereum JSON-RPC and collects
// blocks, transactions, and accounts.
type ChainScanner struct {
	client *rpc.Client
}

// NewChainScanner returns a ChainScanner backed by the given RPC endpoint.
func NewChainScanner(endpoint string) *ChainScanner {
	return &ChainScanner{
		client: rpc.NewClient(endpoint, 30*time.Second),
	}
}

// rpcRawBlock is the minimal subset of an Ethereum block used for scanning.
type rpcRawBlock struct {
	Number       string      `json:"number"`
	Transactions []rpcRawTx  `json:"transactions"`
}

// rpcRawTx is the minimal subset of an Ethereum transaction used for scanning.
type rpcRawTx struct {
	Hash string  `json:"hash"`
	From string  `json:"from"`
	To   *string `json:"to"`
}

// hexToInt64 parses a "0x…" hex string to int64.
func hexToInt64(h string) (int64, error) {
	if len(h) < 2 || h[0] != '0' || h[1] != 'x' {
		return 0, fmt.Errorf("invalid hex string: %q", h)
	}
	n, err := strconv.ParseInt(h[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parse hex %q: %w", h, err)
	}
	return n, nil
}

// LatestBlockNumber calls eth_blockNumber and returns the current chain head.
func (s *ChainScanner) LatestBlockNumber(ctx context.Context) (int64, error) {
	resp, _, err := s.client.Call(ctx, "eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, fmt.Errorf("eth_blockNumber: %w", err)
	}
	var hexNum string
	if err := json.Unmarshal(resp.Result, &hexNum); err != nil {
		return 0, fmt.Errorf("parse block number: %w", err)
	}
	return hexToInt64(hexNum)
}

// Scan iterates blocks from toBlock down to fromBlock (inclusive) and collects
// up to maxBlocks blocks (those with at least one transaction), up to maxTxs
// transactions, and up to maxAccounts unique accounts (sorted by observed tx
// count descending). Scanning stops early once all three limits are satisfied.
func (s *ChainScanner) Scan(
	ctx context.Context,
	fromBlock, toBlock int64,
	maxAccounts, maxTxs, maxBlocks int,
) ([]Account, []Transaction, []Block, error) {
	addrTxCount := make(map[string]int64)
	seenTxs := make(map[string]bool)
	var txs []Transaction
	var blocks []Block

	for blockNum := toBlock; blockNum >= fromBlock; blockNum-- {
		if ctx.Err() != nil {
			return buildAccounts(addrTxCount, maxAccounts), txs, blocks, ctx.Err()
		}

		hexNum := "0x" + strconv.FormatInt(blockNum, 16)
		resp, _, err := s.client.Call(ctx, "eth_getBlockByNumber", []interface{}{hexNum, true})
		if err != nil {
			return buildAccounts(addrTxCount, maxAccounts), txs, blocks,
				fmt.Errorf("fetch block %d: %w", blockNum, err)
		}

		// Null result means the block does not exist yet.
		if len(resp.Result) == 0 || bytes.Equal(resp.Result, []byte("null")) {
			continue
		}

		var block rpcRawBlock
		if err := json.Unmarshal(resp.Result, &block); err != nil {
			return buildAccounts(addrTxCount, maxAccounts), txs, blocks,
				fmt.Errorf("parse block %d: %w", blockNum, err)
		}

		txCount := len(block.Transactions)
		if txCount > 0 {
			if len(blocks) < maxBlocks {
				blocks = append(blocks, Block{Number: blockNum, TxCount: txCount})
			}

			for _, tx := range block.Transactions {
				if seenTxs[tx.Hash] {
					continue
				}
				seenTxs[tx.Hash] = true

				toAddr := ""
				if tx.To != nil {
					toAddr = *tx.To
				}

				if len(txs) < maxTxs {
					txs = append(txs, Transaction{
						Hash:        tx.Hash,
						BlockNumber: blockNum,
						From:        tx.From,
						To:          toAddr,
					})
				}

				// Always track account tx counts regardless of txs limit.
				addrTxCount[tx.From]++
				if toAddr != "" {
					addrTxCount[toAddr]++
				}
			}
		}

		// Stop early once all three limits are satisfied.
		if len(blocks) >= maxBlocks && len(txs) >= maxTxs && len(addrTxCount) >= maxAccounts {
			break
		}
	}

	return buildAccounts(addrTxCount, maxAccounts), txs, blocks, nil
}

// buildAccounts converts the address→txCount map into a slice sorted by
// observed tx count (descending), trimmed to at most limit entries.
func buildAccounts(counts map[string]int64, limit int) []Account {
	accounts := make([]Account, 0, len(counts))
	for addr, cnt := range counts {
		accounts = append(accounts, Account{Address: addr, TxCount: cnt})
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].TxCount > accounts[j].TxCount
	})
	if len(accounts) > limit {
		accounts = accounts[:limit]
	}
	return accounts
}
