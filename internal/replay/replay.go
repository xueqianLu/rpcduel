// Package replay implements the diff-test logic: it loads a dataset, generates
// RPC calls per entity (account / transaction / block), runs them against two
// endpoints concurrently, and categorises the differences.
package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// DiffCategory classifies the kind of mismatch.
type DiffCategory string

const (
	CategoryBalance DiffCategory = "balance_mismatch"
	CategoryNonce   DiffCategory = "nonce_mismatch"
	CategoryTx      DiffCategory = "tx_mismatch"
	CategoryReceipt DiffCategory = "receipt_mismatch"
	CategoryTrace   DiffCategory = "trace_mismatch"
	CategoryMissing DiffCategory = "missing_data"
	CategoryRPCError DiffCategory = "rpc_error"
	CategoryBlock   DiffCategory = "block_mismatch"
)

// archiveErrors is the set of substrings that indicate an archive/pruned node.
var archiveErrors = []string{
	"missing trie node",
	"state not found",
}

// isArchiveError returns true when the error message indicates the node does
// not have historical state (rather than a genuine mismatch).
func isArchiveError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, s := range archiveErrors {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// FoundDiff records one discovered inconsistency.
type FoundDiff struct {
	Category DiffCategory
	Method   string
	Params   []interface{}
	Detail   string
}

// Result holds the aggregate outcome of a diff-test run.
type Result struct {
	AccountsTested     int
	TransactionsTested int
	BlocksTested       int
	TotalRequests      int
	SuccessRequests    int
	Unsupported        int
	Diffs              []FoundDiff
}

// SuccessRate returns the fraction of requests that completed without RPC error.
func (r *Result) SuccessRate() float64 {
	if r.TotalRequests == 0 {
		return 0
	}
	return float64(r.SuccessRequests) / float64(r.TotalRequests)
}

// Summary builds the per-category count map.
func (r *Result) Summary() map[DiffCategory]int {
	m := make(map[DiffCategory]int)
	for _, d := range r.Diffs {
		m[d.Category]++
	}
	return m
}

// Config holds the parameters for a diff-test run.
type Config struct {
	EndpointA       string
	EndpointB       string
	MaxTxPerAccount int
	DiffOpts        diff.Options
}

// Run executes the full diff-test suite against ds.
func Run(ctx context.Context, ds *dataset.Dataset, epA, epB string, maxTxPerAccount int, opts diff.Options) (*Result, error) {
	const requestTimeout = 30 * time.Second
	cA := rpc.NewClient(epA, requestTimeout)
	cB := rpc.NewClient(epB, requestTimeout)

	result := &Result{}

	// Build a lookup: address → list of block numbers from dataset transactions
	addrBlocks := make(map[string][]int64)
	for _, tx := range ds.Transactions {
		addrBlocks[strings.ToLower(tx.From)] = append(addrBlocks[strings.ToLower(tx.From)], tx.BlockNumber)
	}

	// -----------------------------------------------------------------------
	// 1. Account dimension: eth_getBalance + eth_getTransactionCount
	// -----------------------------------------------------------------------
	for _, account := range ds.Accounts {
		result.AccountsTested++
		addr := account.Address

		// Collect block numbers to query; fall back to "latest" if none.
		blockNums := addrBlocks[strings.ToLower(addr)]
		if maxTxPerAccount > 0 && len(blockNums) > maxTxPerAccount {
			blockNums = blockNums[:maxTxPerAccount]
		}
		if len(blockNums) == 0 {
			blockNums = []int64{-1} // -1 sentinel → use "latest"
		}

		for _, bn := range blockNums {
			var blockParam interface{}
			if bn < 0 {
				blockParam = "latest"
			} else {
				blockParam = fmt.Sprintf("0x%x", bn)
			}

			// eth_getBalance
			params := []interface{}{addr, blockParam}
			if d := callAndDiff(ctx, cA, cB, "eth_getBalance", params, opts, CategoryBalance, result); d != nil {
				result.Diffs = append(result.Diffs, *d)
			}

			// eth_getTransactionCount
			if d := callAndDiff(ctx, cA, cB, "eth_getTransactionCount", params, opts, CategoryNonce, result); d != nil {
				result.Diffs = append(result.Diffs, *d)
			}
		}
	}

	// -----------------------------------------------------------------------
	// 2. Transaction dimension: eth_getTransactionByHash + eth_getTransactionReceipt
	// -----------------------------------------------------------------------
	for _, tx := range ds.Transactions {
		result.TransactionsTested++
		params := []interface{}{tx.Hash}

		if d := callAndDiff(ctx, cA, cB, "eth_getTransactionByHash", params, opts, CategoryTx, result); d != nil {
			result.Diffs = append(result.Diffs, *d)
		}
		if d := callAndDiff(ctx, cA, cB, "eth_getTransactionReceipt", params, opts, CategoryReceipt, result); d != nil {
			result.Diffs = append(result.Diffs, *d)
		}
	}

	// -----------------------------------------------------------------------
	// 3. Block dimension: eth_getBlockByNumber
	// -----------------------------------------------------------------------
	for _, block := range ds.Blocks {
		result.BlocksTested++
		params := []interface{}{fmt.Sprintf("0x%x", block.Number), false}

		if d := callAndDiff(ctx, cA, cB, "eth_getBlockByNumber", params, opts, CategoryBlock, result); d != nil {
			result.Diffs = append(result.Diffs, *d)
		}
	}

	return result, nil
}

// callAndDiff sends the same call to both clients and returns a FoundDiff if
// the responses differ. It returns nil when they match or both error identically.
// result.TotalRequests and result.SuccessRequests are updated in-place.
func callAndDiff(ctx context.Context, cA, cB *rpc.Client, method string, params []interface{}, opts diff.Options, cat DiffCategory, result *Result) *FoundDiff {
	respA, _, errA := cA.Call(ctx, method, params)
	respB, _, errB := cB.Call(ctx, method, params)

	result.TotalRequests++

	// Archive/pruned node detection: if either error looks like a missing-state
	// error, mark as unsupported and do not count this as a diff.
	if isArchiveError(errA) || isArchiveError(errB) {
		result.Unsupported++
		return nil
	}

	if errA != nil && errB != nil {
		// Both failed — not a diff.
		return nil
	}

	if errA != nil || errB != nil {
		return &FoundDiff{
			Category: CategoryRPCError,
			Method:   method,
			Params:   params,
			Detail:   fmt.Sprintf("one endpoint errored: %v vs %v", errA, errB),
		}
	}

	// Both endpoints responded successfully.
	result.SuccessRequests++

	// Check for missing / null result on one side.
	aIsNull := isNull(respA)
	bIsNull := isNull(respB)
	if aIsNull != bIsNull {
		return &FoundDiff{
			Category: CategoryMissing,
			Method:   method,
			Params:   params,
			Detail:   fmt.Sprintf("one endpoint returned null: left=%v right=%v", aIsNull, bIsNull),
		}
	}
	if aIsNull && bIsNull {
		return nil
	}

	diffs, err := diff.Compare(respA.Result, respB.Result, opts)
	if err != nil || len(diffs) == 0 {
		return nil
	}
	return &FoundDiff{
		Category: cat,
		Method:   method,
		Params:   params,
		Detail:   diffs[0].String(),
	}
}

// isNull reports whether a response has a JSON null result.
func isNull(resp *rpc.Response) bool {
	if resp == nil {
		return true
	}
	s := strings.TrimSpace(string(resp.Result))
	return s == "" || s == "null"
}

// maxSampleDiffs is the maximum number of sample diffs included in reports.
const maxSampleDiffs = 10

// PrintResult writes a human-readable diff-test summary.
func PrintResult(w io.Writer, r *Result) {
	fmt.Fprintf(w, "\nDiff-Test Result\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 40))
	fmt.Fprintf(w, "Accounts tested:     %d\n", r.AccountsTested)
	fmt.Fprintf(w, "Transactions tested: %d\n", r.TransactionsTested)
	fmt.Fprintf(w, "Blocks tested:       %d\n", r.BlocksTested)
	fmt.Fprintf(w, "Total requests:      %d\n", r.TotalRequests)
	fmt.Fprintf(w, "Success rate:        %.1f%%\n", r.SuccessRate()*100)
	fmt.Fprintf(w, "Unsupported:         %d\n", r.Unsupported)
	fmt.Fprintf(w, "Total diffs:         %d\n", len(r.Diffs))
	if len(r.Diffs) > 0 {
		fmt.Fprintf(w, "\nDiff summary:\n")
		for cat, count := range r.Summary() {
			fmt.Fprintf(w, "  - %s: %d\n", cat, count)
		}
		fmt.Fprintf(w, "\nSample diffs (up to %d):\n", maxSampleDiffs)
		limit := maxSampleDiffs
		if len(r.Diffs) < limit {
			limit = len(r.Diffs)
		}
		for _, d := range r.Diffs[:limit] {
			fmt.Fprintf(w, "  [%s] %s: %s\n", d.Category, d.Method, d.Detail)
		}
	}
}

// PrintResultJSON writes a JSON-encoded summary.
func PrintResultJSON(w io.Writer, r *Result) {
	out := struct {
		AccountsTested     int                  `json:"accounts_tested"`
		TransactionsTested int                  `json:"transactions_tested"`
		BlocksTested       int                  `json:"blocks_tested"`
		TotalRequests      int                  `json:"total_requests"`
		SuccessRate        float64              `json:"success_rate"`
		Unsupported        int                  `json:"unsupported"`
		TotalDiffs         int                  `json:"total_diffs"`
		Summary            map[DiffCategory]int `json:"diff_summary"`
		SampleDiffs        []FoundDiff          `json:"sample_diffs,omitempty"`
	}{
		AccountsTested:     r.AccountsTested,
		TransactionsTested: r.TransactionsTested,
		BlocksTested:       r.BlocksTested,
		TotalRequests:      r.TotalRequests,
		SuccessRate:        r.SuccessRate(),
		Unsupported:        r.Unsupported,
		TotalDiffs:         len(r.Diffs),
		Summary:            r.Summary(),
	}
	limit := maxSampleDiffs
	if len(r.Diffs) < limit {
		limit = len(r.Diffs)
	}
	out.SampleDiffs = r.Diffs[:limit]
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
