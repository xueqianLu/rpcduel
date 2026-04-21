// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// AccountStat is a per-address summary used in Stats.
type AccountStat struct {
	Address string `json:"address"`
	TxCount int64  `json:"tx_count"`
}

// Bucket is a histogram bucket: items with TxCount in [Min, Max].
type Bucket struct {
	Min   int64 `json:"min"`
	Max   int64 `json:"max"`
	Count int   `json:"count"`
}

// EstimatedRPC is the projected number of RPC calls the replay/benchgen
// pipeline would issue for a given category, given the dataset contents.
type EstimatedRPC struct {
	Category string `json:"category"`
	Method   string `json:"method"`
	Count    int    `json:"count"`
	Note     string `json:"note,omitempty"`
}

// Stats is the structured summary produced by Inspect.
type Stats struct {
	File             string         `json:"file"`
	Meta             Meta           `json:"meta"`
	Range            Range          `json:"range"`
	Accounts         int            `json:"accounts"`
	Transactions     int            `json:"transactions"`
	Blocks           int            `json:"blocks"`
	BlockSpan        int64          `json:"block_span"`
	UniqueTxHashes   int            `json:"unique_tx_hashes"`
	TopAccounts      []AccountStat  `json:"top_accounts"`
	TxPerAccountHist []Bucket       `json:"tx_per_account_histogram"`
	EstimatedRPC     []EstimatedRPC `json:"estimated_rpc_calls"`
	EstimatedTotal   int            `json:"estimated_rpc_total"`
}

// Inspect builds a Stats summary for ds. file is the source path used purely
// for reporting and may be empty.
func Inspect(file string, ds *Dataset) Stats {
	s := Stats{
		File:         file,
		Meta:         ds.Meta,
		Range:        ds.Range,
		Accounts:     len(ds.Accounts),
		Transactions: len(ds.Transactions),
		Blocks:       len(ds.Blocks),
	}
	if ds.Range.To >= ds.Range.From {
		s.BlockSpan = ds.Range.To - ds.Range.From + 1
	}

	seen := make(map[string]struct{}, len(ds.Transactions))
	for _, tx := range ds.Transactions {
		seen[tx.Hash] = struct{}{}
	}
	s.UniqueTxHashes = len(seen)

	// Top accounts by tx_count (dataset is already sorted on Save, but be
	// defensive in case it was constructed in memory).
	accs := append([]Account(nil), ds.Accounts...)
	sort.Slice(accs, func(i, j int) bool { return accs[i].TxCount > accs[j].TxCount })
	const topN = 10
	limit := topN
	if len(accs) < limit {
		limit = len(accs)
	}
	for i := 0; i < limit; i++ {
		s.TopAccounts = append(s.TopAccounts, AccountStat{
			Address: accs[i].Address,
			TxCount: accs[i].TxCount,
		})
	}

	// Bucketize tx count: 1, 2-5, 6-20, 21-100, 101-1k, 1k+
	buckets := []Bucket{
		{Min: 1, Max: 1},
		{Min: 2, Max: 5},
		{Min: 6, Max: 20},
		{Min: 21, Max: 100},
		{Min: 101, Max: 1000},
		{Min: 1001, Max: -1},
	}
	for _, a := range ds.Accounts {
		for i := range buckets {
			b := &buckets[i]
			if a.TxCount >= b.Min && (b.Max < 0 || a.TxCount <= b.Max) {
				b.Count++
				break
			}
		}
	}
	s.TxPerAccountHist = buckets

	// Estimated RPC call counts (per the replay & benchgen layouts).
	addEst := func(category, method string, n int, note string) {
		s.EstimatedRPC = append(s.EstimatedRPC, EstimatedRPC{
			Category: category, Method: method, Count: n, Note: note,
		})
		s.EstimatedTotal += n
	}
	addEst("balance", "eth_getBalance", len(ds.Accounts), "1 per account (latest)")
	addEst("transaction_count", "eth_getTransactionCount", len(ds.Accounts), "1 per account (latest)")
	addEst("transaction_by_hash", "eth_getTransactionByHash", len(ds.Transactions), "")
	addEst("transaction_receipt", "eth_getTransactionReceipt", len(ds.Transactions), "")
	addEst("block_by_number", "eth_getBlockByNumber", len(ds.Blocks), "")
	addEst("get_logs", "eth_getLogs", len(ds.Blocks), "1 per block, single-block range")
	addEst("trace_transaction", "debug_traceTransaction", len(ds.Transactions), "opt-in")
	addEst("trace_block", "debug_traceBlockByNumber", len(ds.Blocks), "opt-in")

	return s
}

// PrintStats writes a human-readable inspection summary to w.
func PrintStats(w io.Writer, s Stats) {
	bar := strings.Repeat("-", 60)
	fmt.Fprintf(w, "Dataset: %s\n", s.File)
	fmt.Fprintln(w, bar)
	fmt.Fprintf(w, "  schema_version: %d\n", s.Meta.SchemaVersion)
	fmt.Fprintf(w, "  chain:          %s\n", s.Meta.Chain)
	fmt.Fprintf(w, "  rpc:            %s\n", s.Meta.RPC)
	fmt.Fprintf(w, "  generated_at:   %s\n", s.Meta.GeneratedAt)
	fmt.Fprintf(w, "  block range:    %d .. %d  (span %d)\n", s.Range.From, s.Range.To, s.BlockSpan)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Counts\n")
	fmt.Fprintf(w, "  accounts:        %d\n", s.Accounts)
	fmt.Fprintf(w, "  transactions:    %d  (unique hashes: %d)\n", s.Transactions, s.UniqueTxHashes)
	fmt.Fprintf(w, "  blocks:          %d\n", s.Blocks)
	fmt.Fprintln(w)

	if len(s.TopAccounts) > 0 {
		fmt.Fprintln(w, "Top accounts by tx_count")
		for i, a := range s.TopAccounts {
			fmt.Fprintf(w, "  %2d. %s  %d\n", i+1, a.Address, a.TxCount)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "tx_per_account distribution")
	for _, b := range s.TxPerAccountHist {
		var label string
		if b.Max < 0 {
			label = fmt.Sprintf(">= %d", b.Min)
		} else if b.Min == b.Max {
			label = fmt.Sprintf("    %d", b.Min)
		} else {
			label = fmt.Sprintf("%d - %d", b.Min, b.Max)
		}
		fmt.Fprintf(w, "  %-10s  %d\n", label, b.Count)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Estimated RPC calls per replay / benchgen category")
	for _, e := range s.EstimatedRPC {
		note := ""
		if e.Note != "" {
			note = "  (" + e.Note + ")"
		}
		fmt.Fprintf(w, "  %-22s %-28s %8d%s\n", e.Category, e.Method, e.Count, note)
	}
	fmt.Fprintf(w, "  %-22s %-28s %8d\n", "TOTAL", "", s.EstimatedTotal)
}

// PrintStatsJSON writes the stats as indented JSON to w.
func PrintStatsJSON(w io.Writer, s Stats) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}
