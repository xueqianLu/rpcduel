package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/replay"
)

// makeEchoServer returns a test server that always responds with the given result.
func makeEchoServer(t *testing.T, result interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct{ ID int64 `json:"id"` }
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRun_NoDiffs(t *testing.T) {
	srv := makeEchoServer(t, "0x1")

	ds := &dataset.Dataset{
		Accounts:     []dataset.Account{{Address: "0xabc", TxCount: 1}},
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	opts := diff.DefaultOptions()
	result, err := replay.Run(context.Background(), ds, srv.URL, srv.URL, 10, 4, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Both endpoints are identical so there should be no diffs.
	if len(result.Diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d: %v", len(result.Diffs), result.Diffs)
	}
	if result.AccountsTested != 1 {
		t.Errorf("expected 1 account tested, got %d", result.AccountsTested)
	}
	if result.TransactionsTested != 1 {
		t.Errorf("expected 1 tx tested, got %d", result.TransactionsTested)
	}
	if result.BlocksTested != 1 {
		t.Errorf("expected 1 block tested, got %d", result.BlocksTested)
	}
}

func TestRun_WithDiffs(t *testing.T) {
	srvA := makeEchoServer(t, "0x10")
	srvB := makeEchoServer(t, "0x20") // different result

	ds := &dataset.Dataset{
		Accounts: []dataset.Account{{Address: "0xabc", TxCount: 1}},
	}

	opts := diff.DefaultOptions()
	result, err := replay.Run(context.Background(), ds, srvA.URL, srvB.URL, 10, 4, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(result.Diffs) == 0 {
		t.Error("expected at least one diff")
	}
}

func TestResult_Summary(t *testing.T) {
	r := &replay.Result{
		Diffs: []replay.FoundDiff{
			{Category: replay.CategoryBalance, Method: "eth_getBalance"},
			{Category: replay.CategoryBalance, Method: "eth_getBalance"},
			{Category: replay.CategoryReceipt, Method: "eth_getTransactionReceipt"},
		},
	}
	s := r.Summary()
	if s[replay.CategoryBalance] != 2 {
		t.Errorf("expected 2 balance diffs, got %d", s[replay.CategoryBalance])
	}
	if s[replay.CategoryReceipt] != 1 {
		t.Errorf("expected 1 receipt diff, got %d", s[replay.CategoryReceipt])
	}
}

func TestRun_ArchiveNodeError(t *testing.T) {
	// srvA returns a normal result; srvB returns an archive-node-style error.
	srvA := makeEchoServer(t, "0x1")
	srvErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct{ ID int64 `json:"id"` }
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": "missing trie node abc123 (path ...)",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srvErr.Close)

	ds := &dataset.Dataset{
		Accounts: []dataset.Account{{Address: "0xabc", TxCount: 1}},
	}

	opts := diff.DefaultOptions()
	result, err := replay.Run(context.Background(), ds, srvA.URL, srvErr.URL, 10, 4, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Archive-node errors should be marked as unsupported, not as diffs.
	if len(result.Diffs) != 0 {
		t.Errorf("expected 0 diffs for archive-node error, got %d: %v", len(result.Diffs), result.Diffs)
	}
	if result.Unsupported == 0 {
		t.Error("expected Unsupported > 0 for archive-node error")
	}
}

func TestRun_TxCategory(t *testing.T) {
	srvA := makeEchoServer(t, map[string]interface{}{"hash": "0xtx1", "blockNumber": "0xa"})
	srvB := makeEchoServer(t, map[string]interface{}{"hash": "0xtx1", "blockNumber": "0xb"}) // different

	ds := &dataset.Dataset{
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
	}

	opts := diff.DefaultOptions()
	result, err := replay.Run(context.Background(), ds, srvA.URL, srvB.URL, 10, 4, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	found := false
	for _, d := range result.Diffs {
		if d.Category == replay.CategoryTx && d.Method == "eth_getTransactionByHash" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected tx_mismatch diff for eth_getTransactionByHash, got diffs: %v", result.Diffs)
	}
}

func TestRun_SuccessRate(t *testing.T) {
	srv := makeEchoServer(t, "0x1")

	ds := &dataset.Dataset{
		Accounts:     []dataset.Account{{Address: "0xabc", TxCount: 1}},
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	opts := diff.DefaultOptions()
	result, err := replay.Run(context.Background(), ds, srv.URL, srv.URL, 10, 4, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.TotalRequests == 0 {
		t.Error("expected TotalRequests > 0")
	}
	if result.SuccessRate() < 0.99 {
		t.Errorf("expected success rate ~1.0, got %f", result.SuccessRate())
	}
}

func TestRun_UsesPerAccountTxs(t *testing.T) {
	// Account has Transactions populated; Run should use those block numbers
	// instead of deriving them from the global ds.Transactions list.
	// We set a block number in account.Transactions that doesn't appear in
	// ds.Transactions to confirm the per-account list is used.
	srv := makeEchoServer(t, "0x1")

	ds := &dataset.Dataset{
		Accounts: []dataset.Account{
			{
				Address: "0xabc",
				TxCount: 1,
				Transactions: []dataset.Transaction{
					{Hash: "0xtx99", BlockNumber: 999, From: "0xabc", To: "0xdef"},
				},
			},
		},
		// Global list has a different block; per-account list should take priority.
		Transactions: []dataset.Transaction{
			{Hash: "0xtx1", BlockNumber: 1, From: "0xabc", To: "0xdef"},
		},
	}

	opts := diff.DefaultOptions()
	result, err := replay.Run(context.Background(), ds, srv.URL, srv.URL, 10, 4, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Same server on both sides → no diffs regardless; just verify it runs.
	if len(result.Diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(result.Diffs))
	}
	// 2 requests for account (getBalance + getTransactionCount at block 999)
	// 2 requests for global tx (getTransactionByHash + getTransactionReceipt)
	// = 4 total
	if result.TotalRequests != 4 {
		t.Errorf("expected 4 total requests, got %d", result.TotalRequests)
	}
}

func TestPrintResult_Text(t *testing.T) {
	r := &replay.Result{
		AccountsTested:     5,
		TransactionsTested: 10,
		BlocksTested:       3,
		TotalRequests:      50,
		SuccessRequests:    48,
		Unsupported:        1,
		Diffs: []replay.FoundDiff{
			{Category: replay.CategoryBalance, Method: "eth_getBalance", Detail: "mismatch"},
		},
	}
	var buf bytes.Buffer
	replay.PrintResult(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Accounts tested:") {
		t.Error("expected 'Accounts tested:' in text output")
	}
	if !strings.Contains(out, "balance_mismatch") {
		t.Error("expected 'balance_mismatch' in text output")
	}
}

func TestPrintResultJSON_ContainsFields(t *testing.T) {
	r := &replay.Result{
		AccountsTested:     2,
		TransactionsTested: 3,
		BlocksTested:       1,
		TotalRequests:      10,
		SuccessRequests:    10,
	}
	var buf bytes.Buffer
	replay.PrintResultJSON(&buf, r)
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("PrintResultJSON produced invalid JSON: %v", err)
	}
	for _, key := range []string{"accounts_tested", "transactions_tested", "blocks_tested", "total_requests", "success_rate"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected key %q in JSON output", key)
		}
	}
}
