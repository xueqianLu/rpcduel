// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package replay implements the replay logic: it loads a dataset, generates
// RPC calls per entity (account / transaction / block), runs them against two
// endpoints concurrently, and categorizes the differences.
package replay

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// progressInterval is the number of completed tasks between progress log lines.
// A final line is always emitted when all tasks have finished.
const progressInterval = 100

// DiffCategory classifies the kind of mismatch.
type DiffCategory string

const (
	CategoryBalance  DiffCategory = "balance_mismatch"
	CategoryNonce    DiffCategory = "nonce_mismatch"
	CategoryTx       DiffCategory = "tx_mismatch"
	CategoryReceipt  DiffCategory = "receipt_mismatch"
	CategoryTrace    DiffCategory = "trace_mismatch"
	CategoryMissing  DiffCategory = "missing_data"
	CategoryRPCError DiffCategory = "rpc_error"
	CategoryBlock    DiffCategory = "block_mismatch"
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

// Result holds the aggregate outcome of a replay run.
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

// Config holds the parameters for a replay run.
type Config struct {
	EndpointA        string
	EndpointB        string
	MaxTxPerAccount  int
	DiffOpts         diff.Options
	TraceTransaction bool
	TraceBlock       bool
	Only             map[string]bool
	// RPCOptions controls the underlying RPC client behavior (timeout,
	// retries, headers, ...). When zero-valued, sensible defaults are used.
	RPCOptions rpc.Options

	// StateFile, when non-empty, enables periodic checkpointing of replay
	// progress to this path so an interrupted run can be resumed via Resume.
	StateFile string
	// Resume, when true and StateFile points to an existing file, loads
	// previously-completed task keys + counters and continues from there.
	Resume bool
	// StateInterval is the number of completed tasks between state flushes.
	// Defaults to 100 when <= 0.
	StateInterval int
	// DatasetPath is recorded in the state file (informational only) so
	// resumes against the wrong dataset can be flagged.
	DatasetPath string
}

// Run executes the full replay suite against ds using concurrency goroutines.
// If concurrency is <= 0 it defaults to 1.
// progress, if non-nil, receives periodic one-line status updates written as
// each batch of tasks completes (every progressInterval tasks and at the end).
func Run(ctx context.Context, ds *dataset.Dataset, cfg Config, concurrency int, progress io.Writer) (*Result, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	const requestTimeout = 30 * time.Second
	rpcOpts := cfg.RPCOptions
	if rpcOpts.Timeout <= 0 {
		rpcOpts.Timeout = requestTimeout
	}
	cA := rpc.NewClientWithOptions(cfg.EndpointA, rpcOpts)
	cB := rpc.NewClientWithOptions(cfg.EndpointB, rpcOpts)

	result := &Result{
		AccountsTested:     0,
		TransactionsTested: 0,
		BlocksTested:       0,
	}
	accountEnabled := cfg.enabled("balance") || cfg.enabled("transaction_count")
	transactionEnabled := cfg.enabled("transaction_by_hash") || cfg.enabled("transaction_receipt") || cfg.enabled("trace_transaction")
	blockEnabled := cfg.enabled("block_by_number") || cfg.enabled("trace_block")
	if accountEnabled {
		result.AccountsTested = len(ds.Accounts)
	}
	if transactionEnabled {
		result.TransactionsTested = len(ds.Transactions)
	}
	if blockEnabled {
		result.BlocksTested = len(ds.Blocks)
	}

	// Build a lookup: address → list of block numbers from dataset transactions.
	// Used as fallback when Account.Transactions is not populated (older datasets).
	addrBlocks := make(map[string][]int64)
	addrBlockSeen := make(map[string]map[int64]bool)
	for _, tx := range ds.Transactions {
		addr := strings.ToLower(tx.From)
		if addrBlockSeen[addr] == nil {
			addrBlockSeen[addr] = make(map[int64]bool)
		}
		if !addrBlockSeen[addr][tx.BlockNumber] {
			addrBlockSeen[addr][tx.BlockNumber] = true
			addrBlocks[addr] = append(addrBlocks[addr], tx.BlockNumber)
		}
	}

	type rpcTask struct {
		method string
		params []interface{}
		cat    DiffCategory
	}

	// Build the full ordered task list.
	var tasks []rpcTask
	seenTasks := make(map[string]bool)
	addTask := func(method string, params []interface{}, cat DiffCategory) {
		key, err := taskKey(method, params)
		if err != nil {
			// Should not happen for the small JSON-compatible param shapes used here.
			// Fall back to a simple fmt-based key so we do not silently drop coverage.
			key = fmt.Sprintf("%s|%v", method, params)
		}
		if seenTasks[key] {
			return
		}
		seenTasks[key] = true
		tasks = append(tasks, rpcTask{method: method, params: params, cat: cat})
	}

	// 1. Account dimension: eth_getBalance + eth_getTransactionCount
	for _, account := range ds.Accounts {
		addr := account.Address

		// Prefer per-account transactions stored in the dataset; fall back to
		// deriving block numbers from the global transaction list.
		var blockNums []int64
		if len(account.Transactions) > 0 {
			seen := make(map[int64]bool)
			for _, tx := range account.Transactions {
				if !seen[tx.BlockNumber] {
					seen[tx.BlockNumber] = true
					blockNums = append(blockNums, tx.BlockNumber)
				}
			}
		} else {
			blockNums = addrBlocks[strings.ToLower(addr)]
		}

		if cfg.MaxTxPerAccount > 0 && len(blockNums) > cfg.MaxTxPerAccount {
			blockNums = blockNums[:cfg.MaxTxPerAccount]
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
			params := []interface{}{addr, blockParam}
			if cfg.enabled("balance") {
				addTask("eth_getBalance", params, CategoryBalance)
			}
			if cfg.enabled("transaction_count") {
				addTask("eth_getTransactionCount", params, CategoryNonce)
			}
		}
	}

	// 2. Transaction dimension: eth_getTransactionByHash + eth_getTransactionReceipt
	for _, tx := range ds.Transactions {
		params := []interface{}{tx.Hash}
		if cfg.enabled("transaction_by_hash") {
			addTask("eth_getTransactionByHash", params, CategoryTx)
		}
		if cfg.enabled("transaction_receipt") {
			addTask("eth_getTransactionReceipt", params, CategoryReceipt)
		}
		if cfg.enabled("trace_transaction") {
			addTask("debug_traceTransaction", []interface{}{tx.Hash, map[string]interface{}{}}, CategoryTrace)
		}
	}

	// 3. Block dimension: eth_getBlockByNumber + debug_traceBlockByNumber
	for _, block := range ds.Blocks {
		hexNumber := fmt.Sprintf("0x%x", block.Number)
		if cfg.enabled("block_by_number") {
			addTask("eth_getBlockByNumber", []interface{}{hexNumber, false}, CategoryBlock)
		}
		if cfg.enabled("trace_block") {
			addTask("debug_traceBlockByNumber", []interface{}{hexNumber, map[string]interface{}{}}, CategoryTrace)
		}
	}

	taskCh := make(chan rpcTask, len(tasks))
	skipped := 0
	resume := cfg.Resume && cfg.StateFile != ""
	var seed *State
	if resume {
		s, err := LoadState(cfg.StateFile)
		if err == nil {
			seed = s
			if s.EndpointA != "" && (s.EndpointA != cfg.EndpointA || s.EndpointB != cfg.EndpointB) {
				return nil, fmt.Errorf("resume: state was for endpoints (%s,%s) but current run uses (%s,%s)",
					s.EndpointA, s.EndpointB, cfg.EndpointA, cfg.EndpointB)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("resume: load state: %w", err)
		}
	}
	if seed == nil {
		seed = &State{
			EndpointA:   cfg.EndpointA,
			EndpointB:   cfg.EndpointB,
			DatasetPath: cfg.DatasetPath,
		}
	} else {
		// Pre-populate counters/diffs into the result.
		result.TotalRequests = seed.TotalRequests
		result.SuccessRequests = seed.SuccessRequests
		result.Unsupported = seed.Unsupported
		result.Diffs = append(result.Diffs, seed.Diffs...)
	}
	recorder := newStateRecorder(cfg.StateFile, seed)

	enqueued := 0
	for _, t := range tasks {
		key, _ := taskKey(t.method, t.params)
		if recorder.isDone(key) {
			skipped++
			continue
		}
		taskCh <- t
		enqueued++
	}
	close(taskCh)
	if resume && progress != nil {
		fmt.Fprintf(progress, "Resume: skipping %d already-completed tasks; running %d remaining\n", skipped, enqueued)
	}

	type workItem struct {
		t   rpcTask
		key string
	}
	workCh := make(chan workItem, concurrency)
	go func() {
		defer close(workCh)
		for t := range taskCh {
			key, _ := taskKey(t.method, t.params)
			workCh <- workItem{t: t, key: key}
		}
	}()

	type doneItem struct {
		key string
		out callOutcome
	}
	resCh := make(chan doneItem, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range workCh {
				resCh <- doneItem{key: w.key, out: callAndDiff(ctx, cA, cB, w.t.method, w.t.params, cfg.DiffOpts, w.t.cat)}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	flushEvery := cfg.StateInterval
	if flushEvery <= 0 {
		flushEvery = 100
	}

	total := enqueued
	done := 0
	for di := range resCh {
		done++
		out := di.out
		result.TotalRequests += out.totalReqs
		result.SuccessRequests += out.successReqs
		result.Unsupported += out.unsupported
		if out.diff != nil {
			result.Diffs = append(result.Diffs, *out.diff)
		}
		recorder.record(di.key, out)
		if recorder.shouldFlush(flushEvery) {
			if err := recorder.flush(); err != nil {
				slogWarnState(progress, err)
			}
		}
		if progress != nil && total > 0 && (done == total || (done > 0 && done%progressInterval == 0)) {
			pct := float64(done) / float64(total) * 100
			fmt.Fprintf(progress, "Progress: %d/%d tasks (%.1f%%)\n", done, total, pct)
		}
	}

	// Final flush so a clean run leaves the state file consistent.
	if err := recorder.flush(); err != nil {
		slogWarnState(progress, err)
	}

	return result, nil
}

// slogWarnState reports a state-flush failure non-fatally. We prefer the
// progress writer for visibility; in production this is os.Stderr.
func slogWarnState(progress io.Writer, err error) {
	if progress != nil {
		fmt.Fprintf(progress, "warning: state flush failed: %v\n", err)
	}
}

func (c Config) enabled(target string) bool {
	if len(c.Only) > 0 {
		return c.Only[target]
	}
	switch target {
	case "trace_transaction":
		return c.TraceTransaction
	case "trace_block":
		return c.TraceBlock
	default:
		return true
	}
}

// callOutcome carries the counters and optional diff produced by one RPC pair.
type callOutcome struct {
	totalReqs   int
	successReqs int
	unsupported int
	diff        *FoundDiff
}

// callAndDiff sends the same call to both clients and returns a callOutcome
// describing whether the responses match. Responses are compared only when
// both endpoints succeed (non-archive-node error).
func callAndDiff(ctx context.Context, cA, cB *rpc.Client, method string, params []interface{}, opts diff.Options, cat DiffCategory) callOutcome {
	respA, _, errA := cA.Call(ctx, method, params)
	respB, _, errB := cB.Call(ctx, method, params)

	out := callOutcome{totalReqs: 1}

	// Archive/pruned node detection: if either error looks like a missing-state
	// error, mark as unsupported and do not count this as a diff.
	if isArchiveError(errA) || isArchiveError(errB) {
		out.unsupported = 1
		return out
	}

	if errA != nil && errB != nil {
		// Both failed — not a diff.
		return out
	}

	if errA != nil || errB != nil {
		out.diff = &FoundDiff{
			Category: CategoryRPCError,
			Method:   method,
			Params:   params,
			Detail:   fmt.Sprintf("one endpoint errored: %v vs %v", errA, errB),
		}
		return out
	}

	// Both endpoints responded successfully.
	out.successReqs = 1

	// Check for missing / null result on one side.
	aIsNull := isNull(respA)
	bIsNull := isNull(respB)
	if aIsNull != bIsNull {
		out.diff = &FoundDiff{
			Category: CategoryMissing,
			Method:   method,
			Params:   params,
			Detail:   fmt.Sprintf("one endpoint returned null: left=%v right=%v", aIsNull, bIsNull),
		}
		return out
	}
	if aIsNull && bIsNull {
		return out
	}

	diffs, err := diff.Compare(respA.Result, respB.Result, opts)
	if err != nil || len(diffs) == 0 {
		return out
	}
	out.diff = &FoundDiff{
		Category: cat,
		Method:   method,
		Params:   params,
		Detail:   diffs[0].String(),
	}
	return out
}

// isNull reports whether a response has a JSON null result.
func isNull(resp *rpc.Response) bool {
	if resp == nil {
		return true
	}
	s := strings.TrimSpace(string(resp.Result))
	return s == "" || s == "null"
}

func taskKey(method string, params []interface{}) (string, error) {
	canonical, err := canonicalJSON(params)
	if err != nil {
		return "", err
	}
	return method + "|" + canonical, nil
}

func canonicalJSON(v interface{}) (string, error) {
	b, err := marshalCanonical(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func marshalCanonical(v interface{}) ([]byte, error) {
	switch x := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			keyJSON, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			valJSON, err := marshalCanonical(x[k])
			if err != nil {
				return nil, err
			}
			parts = append(parts, string(keyJSON)+":"+string(valJSON))
		}
		return []byte("{" + strings.Join(parts, ",") + "}"), nil
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			itemJSON, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			parts = append(parts, string(itemJSON))
		}
		return []byte("[" + strings.Join(parts, ",") + "]"), nil
	default:
		return json.Marshal(v)
	}
}

// maxSampleDiffs is the maximum number of sample diffs included in reports.
const maxSampleDiffs = 10

// WriteResultCSV writes all discovered diffs to w as a CSV file.
// The CSV has four columns: category, method, params, detail.
// The first row is a header.  Returns the first encoding or flush error.
func WriteResultCSV(w io.Writer, r *Result) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"category", "method", "params", "detail"}); err != nil {
		return err
	}
	for _, d := range r.Diffs {
		params, _ := json.Marshal(d.Params)
		if err := cw.Write([]string{
			string(d.Category),
			d.Method,
			string(params),
			d.Detail,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// PrintResult writes a human-readable replay summary.
func PrintResult(w io.Writer, r *Result) {
	fmt.Fprintf(w, "\nReplay Result\n")
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
