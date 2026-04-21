// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

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
	"sync"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// ChainScanner scans a block range via Ethereum JSON-RPC and collects
// blocks, transactions, and accounts.
type ChainScanner struct {
	client *rpc.Client
}

// NewChainScanner returns a ChainScanner backed by the given RPC endpoint
// using default request options (30s timeout, no retries).
func NewChainScanner(endpoint string) *ChainScanner {
	return &ChainScanner{
		client: rpc.NewClient(endpoint, 30*time.Second),
	}
}

// NewChainScannerWithOptions returns a ChainScanner backed by the given RPC
// endpoint using the supplied client options (allows retries, headers, etc.).
func NewChainScannerWithOptions(endpoint string, opts rpc.Options) *ChainScanner {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	return &ChainScanner{
		client: rpc.NewClientWithOptions(endpoint, opts),
	}
}

// rpcRawBlock is the minimal subset of an Ethereum block used for scanning.
type rpcRawBlock struct {
	Number       string     `json:"number"`
	Transactions []rpcRawTx `json:"transactions"`
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

// Scan iterates blocks from toBlock down to fromBlock (inclusive) using
// concurrency goroutines in parallel and collects up to maxBlocks blocks
// (those with at least one transaction), up to maxTxs transactions, and up
// to maxAccounts unique accounts (sorted by observed tx count descending).
// maxTxPerAccount limits the number of transactions stored per account in
// Account.Transactions (0 = unlimited).
// Scanning stops early once all three limits are satisfied.
// If concurrency is <= 0 it defaults to 1.
func (s *ChainScanner) Scan(
	ctx context.Context,
	fromBlock, toBlock int64,
	maxAccounts, maxTxs, maxBlocks int,
	maxTxPerAccount int,
	concurrency int,
) ([]Account, []Transaction, []Block, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type blockResult struct {
		num   int64
		block *rpcRawBlock // nil means null / skipped block
		err   error
	}

	blockNums := make(chan int64, concurrency)
	results := make(chan blockResult, concurrency)

	// Start worker goroutines.
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for blockNum := range blockNums {
				hexNum := "0x" + strconv.FormatInt(blockNum, 16)
				resp, _, err := s.client.Call(ctx, "eth_getBlockByNumber", []interface{}{hexNum, true})

				var br blockResult
				br.num = blockNum
				if err != nil {
					br.err = fmt.Errorf("fetch block %d: %w", blockNum, err)
				} else if len(resp.Result) == 0 || bytes.Equal(resp.Result, []byte("null")) {
					// null block – skip
				} else {
					var block rpcRawBlock
					if err := json.Unmarshal(resp.Result, &block); err != nil {
						br.err = fmt.Errorf("parse block %d: %w", blockNum, err)
					} else {
						br.block = &block
					}
				}

				select {
				case results <- br:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Close results channel after all workers finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Feed block numbers from high to low; exit early on cancellation.
	go func() {
		defer close(blockNums)
		for blockNum := toBlock; blockNum >= fromBlock; blockNum-- {
			select {
			case blockNums <- blockNum:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results in the main goroutine (no lock needed).
	addrTxCount := make(map[string]int64)
	addrTxs := make(map[string][]Transaction) // per-account transaction list
	seenTxs := make(map[string]bool)
	var txs []Transaction
	var blocks []Block
	var firstErr error

	for result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			cancel()
			break
		}

		if result.block == nil {
			continue // null / empty block
		}

		block := result.block
		blockNum := result.num

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

				rec := Transaction{
					Hash:        tx.Hash,
					BlockNumber: blockNum,
					From:        tx.From,
					To:          toAddr,
				}

				if len(txs) < maxTxs {
					txs = append(txs, rec)
				}

				// Always track account tx counts regardless of txs limit.
				addrTxCount[tx.From]++
				if toAddr != "" {
					addrTxCount[toAddr]++
				}

				// Store per-account transactions up to maxTxPerAccount.
				if maxTxPerAccount == 0 || len(addrTxs[tx.From]) < maxTxPerAccount {
					addrTxs[tx.From] = append(addrTxs[tx.From], rec)
				}
				if toAddr != "" {
					if maxTxPerAccount == 0 || len(addrTxs[toAddr]) < maxTxPerAccount {
						addrTxs[toAddr] = append(addrTxs[toAddr], rec)
					}
				}
			}
		}

		// Stop early once all three limits are satisfied.
		if len(blocks) >= maxBlocks && len(txs) >= maxTxs && len(addrTxCount) >= maxAccounts {
			cancel()
			break
		}
	}

	// Drain any remaining results so workers / closer goroutines can exit.
	for range results {
	}

	return buildAccounts(addrTxCount, addrTxs, maxAccounts), txs, blocks, firstErr
}

// buildAccounts converts the address→txCount map into a slice sorted by
// observed tx count (descending), trimmed to at most limit entries.
// addrTxs provides the per-account transaction list stored in Account.Transactions.
func buildAccounts(counts map[string]int64, addrTxs map[string][]Transaction, limit int) []Account {
	accounts := make([]Account, 0, len(counts))
	for addr, cnt := range counts {
		accounts = append(accounts, Account{
			Address:      addr,
			TxCount:      cnt,
			Transactions: addrTxs[addr],
		})
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].TxCount > accounts[j].TxCount
	})
	if len(accounts) > limit {
		accounts = accounts[:limit]
	}
	return accounts
}
