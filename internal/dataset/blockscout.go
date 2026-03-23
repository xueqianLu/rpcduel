// Package dataset – Blockscout REST API v2 client with pagination, retry, and
// rate limiting.
package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultRateLimit = 5               // requests per second
	defaultRetries   = 3
	defaultBackoff   = 500 * time.Millisecond
	pageSize         = 50
)

// BlockscoutClient fetches chain data from a Blockscout v2 REST API.
type BlockscoutClient struct {
	base    string
	http    *http.Client
	limiter <-chan time.Time
	retries int
}

// NewBlockscoutClient returns a client for the given Blockscout base URL.
// ratePerSec controls the maximum request rate (default 5).
func NewBlockscoutClient(baseURL string, ratePerSec int) *BlockscoutClient {
	if ratePerSec <= 0 {
		ratePerSec = defaultRateLimit
	}
	interval := time.Second / time.Duration(ratePerSec)
	ticker := time.NewTicker(interval)
	return &BlockscoutClient{
		base:    baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: ticker.C,
		retries: defaultRetries,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (c *BlockscoutClient) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.limiter:
		return nil
	}
}

func (c *BlockscoutClient) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var (
		data []byte
		err  error
	)
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(defaultBackoff * time.Duration(attempt)):
			}
		}
		if werr := c.wait(ctx); werr != nil {
			return nil, werr
		}
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if rerr != nil {
			return nil, fmt.Errorf("create request: %w", rerr)
		}
		req.Header.Set("Accept", "application/json")

		resp, herr := c.http.Do(req)
		if herr != nil {
			err = fmt.Errorf("http get %s: %w", u, herr)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			err = fmt.Errorf("rate limited by server")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			err = fmt.Errorf("http %d from %s", resp.StatusCode, u)
			continue
		}
		data, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			err = fmt.Errorf("read body: %w", err)
			continue
		}
		return data, nil
	}
	return nil, err
}

// ---------------------------------------------------------------------------
// Blockscout v2 response shapes (minimal, only fields we need)
// ---------------------------------------------------------------------------

type bsPagedResponse struct {
	Items         json.RawMessage `json:"items"`
	NextPageParams json.RawMessage `json:"next_page_params"`
}

type bsAddress struct {
	Hash             string `json:"hash"`
	TransactionCount int64  `json:"transactions_count,string"`
}

type bsTx struct {
	Hash   string `json:"hash"`
	Block  int64  `json:"block_number"`
	From   bsAddressRef `json:"from"`
	To     *bsAddressRef `json:"to"`
	Status string `json:"status"`
}

type bsAddressRef struct {
	Hash string `json:"hash"`
}

type bsBlock struct {
	Height  int64 `json:"height"`
	TxCount int   `json:"transaction_count"`
}

// bsNextPage holds common next-page cursor fields Blockscout v2 returns.
type bsNextPage struct {
	PageNumber  int    `json:"page_number,omitempty"`
	BlockNumber int64  `json:"block_number,omitempty"`
	Index       int64  `json:"index,omitempty"`
	InsertedAt  string `json:"inserted_at,omitempty"`
	Hash        string `json:"hash,omitempty"`
	ItemsCount  int    `json:"items_count,omitempty"`
}

// ---------------------------------------------------------------------------
// Public fetch methods
// ---------------------------------------------------------------------------

// FetchAccounts returns up to limit top accounts sorted by transaction count.
func (c *BlockscoutClient) FetchAccounts(ctx context.Context, limit int) ([]Account, error) {
	seen := make(map[string]bool)
	var accounts []Account

	q := url.Values{}
	q.Set("sort", "transactions_count")
	q.Set("order", "desc")

	for len(accounts) < limit {
		data, err := c.get(ctx, "/api/v2/addresses", q)
		if err != nil {
			return accounts, fmt.Errorf("fetch addresses: %w", err)
		}

		var page bsPagedResponse
		if err := json.Unmarshal(data, &page); err != nil {
			return accounts, fmt.Errorf("parse addresses page: %w", err)
		}

		var items []bsAddress
		if err := json.Unmarshal(page.Items, &items); err != nil {
			return accounts, fmt.Errorf("parse address items: %w", err)
		}
		if len(items) == 0 {
			break
		}

		for _, a := range items {
			if seen[a.Hash] {
				continue
			}
			seen[a.Hash] = true
			accounts = append(accounts, Account{Address: a.Hash, TxCount: a.TransactionCount})
			if len(accounts) >= limit {
				break
			}
		}

		// Advance pagination
		if len(page.NextPageParams) == 0 || string(page.NextPageParams) == "null" {
			break
		}
		var next bsNextPage
		if err := json.Unmarshal(page.NextPageParams, &next); err != nil || next.ItemsCount == 0 {
			break
		}
		q = url.Values{}
		q.Set("sort", "transactions_count")
		q.Set("order", "desc")
		q.Set("items_count", strconv.Itoa(next.ItemsCount))
		if next.Hash != "" {
			q.Set("address_hash", next.Hash)
		}
		if next.InsertedAt != "" {
			q.Set("inserted_at", next.InsertedAt)
		}
	}

	return accounts, nil
}

// FetchTransactions returns up to limit transactions in [fromBlock, toBlock].
func (c *BlockscoutClient) FetchTransactions(ctx context.Context, fromBlock, toBlock int64, limit int) ([]Transaction, error) {
	seen := make(map[string]bool)
	var txs []Transaction

	q := url.Values{}
	q.Set("filter", "validated")
	if fromBlock > 0 {
		q.Set("block_number_from", strconv.FormatInt(fromBlock, 10))
	}
	if toBlock > 0 {
		q.Set("block_number_to", strconv.FormatInt(toBlock, 10))
	}

	for len(txs) < limit {
		data, err := c.get(ctx, "/api/v2/transactions", q)
		if err != nil {
			return txs, fmt.Errorf("fetch transactions: %w", err)
		}

		var page bsPagedResponse
		if err := json.Unmarshal(data, &page); err != nil {
			return txs, fmt.Errorf("parse transactions page: %w", err)
		}

		var items []bsTx
		if err := json.Unmarshal(page.Items, &items); err != nil {
			return txs, fmt.Errorf("parse transaction items: %w", err)
		}
		if len(items) == 0 {
			break
		}

		for _, tx := range items {
			if seen[tx.Hash] {
				continue
			}
			// Filter block range
			if fromBlock > 0 && tx.Block < fromBlock {
				continue
			}
			if toBlock > 0 && tx.Block > toBlock {
				continue
			}
			seen[tx.Hash] = true
			toAddr := ""
			if tx.To != nil {
				toAddr = tx.To.Hash
			}
			txs = append(txs, Transaction{
				Hash:        tx.Hash,
				BlockNumber: tx.Block,
				From:        tx.From.Hash,
				To:          toAddr,
			})
			if len(txs) >= limit {
				break
			}
		}

		if len(page.NextPageParams) == 0 || string(page.NextPageParams) == "null" {
			break
		}
		var next bsNextPage
		if err := json.Unmarshal(page.NextPageParams, &next); err != nil {
			break
		}
		q = url.Values{}
		q.Set("filter", "validated")
		if fromBlock > 0 {
			q.Set("block_number_from", strconv.FormatInt(fromBlock, 10))
		}
		if toBlock > 0 {
			q.Set("block_number_to", strconv.FormatInt(toBlock, 10))
		}
		if next.BlockNumber > 0 {
			q.Set("block_number", strconv.FormatInt(next.BlockNumber, 10))
		}
		if next.Index > 0 {
			q.Set("index", strconv.FormatInt(next.Index, 10))
		}
		if next.InsertedAt != "" {
			q.Set("inserted_at", next.InsertedAt)
		}
	}

	return txs, nil
}

// FetchBlocks returns up to limit blocks in [fromBlock, toBlock] that have transactions.
func (c *BlockscoutClient) FetchBlocks(ctx context.Context, fromBlock, toBlock int64, limit int) ([]Block, error) {
	seen := make(map[int64]bool)
	var blocks []Block

	q := url.Values{}
	q.Set("type", "block")

	for len(blocks) < limit {
		data, err := c.get(ctx, "/api/v2/blocks", q)
		if err != nil {
			return blocks, fmt.Errorf("fetch blocks: %w", err)
		}

		var page bsPagedResponse
		if err := json.Unmarshal(data, &page); err != nil {
			return blocks, fmt.Errorf("parse blocks page: %w", err)
		}

		var items []bsBlock
		if err := json.Unmarshal(page.Items, &items); err != nil {
			return blocks, fmt.Errorf("parse block items: %w", err)
		}
		if len(items) == 0 {
			break
		}

		allOutOfRange := true
		for _, b := range items {
			if fromBlock > 0 && b.Height < fromBlock {
				continue
			}
			if toBlock > 0 && b.Height > toBlock {
				continue
			}
			allOutOfRange = false
			if seen[b.Height] {
				continue
			}
			if b.TxCount == 0 {
				continue
			}
			seen[b.Height] = true
			blocks = append(blocks, Block{Number: b.Height, TxCount: b.TxCount})
			if len(blocks) >= limit {
				break
			}
		}

		if allOutOfRange || len(page.NextPageParams) == 0 || string(page.NextPageParams) == "null" {
			break
		}
		var next bsNextPage
		if err := json.Unmarshal(page.NextPageParams, &next); err != nil {
			break
		}
		q = url.Values{}
		q.Set("type", "block")
		if next.BlockNumber > 0 {
			q.Set("block_number", strconv.FormatInt(next.BlockNumber, 10))
		}
		if next.ItemsCount > 0 {
			q.Set("items_count", strconv.Itoa(next.ItemsCount))
		}
	}

	return blocks, nil
}
