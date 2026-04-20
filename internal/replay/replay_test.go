package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/replay"
)

func testConfig(epA, epB string, opts diff.Options) replay.Config {
	return replay.Config{
		EndpointA:       epA,
		EndpointB:       epB,
		MaxTxPerAccount: 10,
		DiffOpts:        opts,
	}
}

// makeEchoServer returns a test server that always responds with the given result.
func makeEchoServer(t *testing.T, result interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ID int64 `json:"id"`
		}
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
	result, err := replay.Run(context.Background(), ds, testConfig(srv.URL, srv.URL, opts), 4, nil)
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
	result, err := replay.Run(context.Background(), ds, testConfig(srvA.URL, srvB.URL, opts), 4, nil)
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
		var req struct {
			ID int64 `json:"id"`
		}
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
	result, err := replay.Run(context.Background(), ds, testConfig(srvA.URL, srvErr.URL, opts), 4, nil)
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
	result, err := replay.Run(context.Background(), ds, testConfig(srvA.URL, srvB.URL, opts), 4, nil)
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
	result, err := replay.Run(context.Background(), ds, testConfig(srv.URL, srv.URL, opts), 4, nil)
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
	result, err := replay.Run(context.Background(), ds, testConfig(srv.URL, srv.URL, opts), 4, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Same server on both sides → no diffs regardless; just verify it runs.
	if len(result.Diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(result.Diffs))
	}
	// 2 requests for account (getBalance + getTransactionCount at block 999)
	// 2 requests for global tx (getTransactionByHash + getTransactionReceipt)
	// = 4 total by default (trace disabled)
	if result.TotalRequests != 4 {
		t.Errorf("expected 4 total requests, got %d", result.TotalRequests)
	}
}

func TestRun_AddsTraceTasks(t *testing.T) {
	type requestShape struct {
		Method string
		Params string
	}
	var mu sync.Mutex
	seen := make(map[requestShape]int)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ID     int64         `json:"id"`
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		paramsJSON, _ := json.Marshal(req.Params)
		mu.Lock()
		seen[requestShape{Method: req.Method, Params: string(paramsJSON)}]++
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]interface{}{"ok": true},
		})
	}

	srvA := httptest.NewServer(http.HandlerFunc(handler))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(handler))
	defer srvB.Close()

	ds := &dataset.Dataset{
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	cfg := testConfig(srvA.URL, srvB.URL, diff.DefaultOptions())
	cfg.TraceTransaction = true
	cfg.TraceBlock = true
	result, err := replay.Run(context.Background(), ds, cfg, 2, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TotalRequests != 5 {
		t.Fatalf("expected 5 replay tasks (tx, receipt, trace tx, block, trace block), got %d", result.TotalRequests)
	}

	for _, want := range []requestShape{
		{Method: "eth_getTransactionByHash", Params: `["0xtx1"]`},
		{Method: "eth_getTransactionReceipt", Params: `["0xtx1"]`},
		{Method: "debug_traceTransaction", Params: `["0xtx1",{}]`},
		{Method: "eth_getBlockByNumber", Params: `["0xa",false]`},
		{Method: "debug_traceBlockByNumber", Params: `["0xa",{}]`},
	} {
		if seen[want] == 0 {
			t.Fatalf("expected request %+v to be sent, seen=%v", want, seen)
		}
	}
}

func TestRun_DeduplicatesSameMethodAndParams(t *testing.T) {
	type requestShape struct {
		Method string
		Params string
	}
	var mu sync.Mutex
	seen := make(map[requestShape]int)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ID     int64         `json:"id"`
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		paramsJSON, _ := json.Marshal(req.Params)
		mu.Lock()
		seen[requestShape{Method: req.Method, Params: string(paramsJSON)}]++
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "0x1",
		})
	}

	srvA := httptest.NewServer(http.HandlerFunc(handler))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(handler))
	defer srvB.Close()

	ds := &dataset.Dataset{
		Accounts: []dataset.Account{{Address: "0xabc", TxCount: 2}},
		Transactions: []dataset.Transaction{
			{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"},
			{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"},
		},
		Blocks: []dataset.Block{{Number: 10, TxCount: 2}, {Number: 10, TxCount: 2}},
	}

	result, err := replay.Run(context.Background(), ds, testConfig(srvA.URL, srvB.URL, diff.DefaultOptions()), 2, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// account: balance + nonce at one deduped block number = 2
	// tx: byHash + receipt = 2
	// block: getBlock = 1
	// total = 5 (account 2 + tx 2 + block 1)
	if result.TotalRequests != 5 {
		t.Fatalf("expected 5 deduplicated tasks, got %d", result.TotalRequests)
	}

	for req, count := range seen {
		if count != 2 {
			t.Fatalf("expected each unique request to hit two endpoints exactly once, got %s %s -> %d", req.Method, req.Params, count)
		}
	}

	// Distinct methods for the same tx/block should still coexist.
	for _, req := range []requestShape{
		{Method: "eth_getTransactionByHash", Params: `["0xtx1"]`},
		{Method: "eth_getTransactionReceipt", Params: `["0xtx1"]`},
		{Method: "eth_getBlockByNumber", Params: `["0xa",false]`},
	} {
		if seen[req] != 2 {
			t.Fatalf("expected distinct method request %s %s to remain, seen=%d", req.Method, req.Params, seen[req])
		}
	}
}

func TestRun_TraceCategory(t *testing.T) {
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := map[string]interface{}{"same": true}
		if req.Method == "debug_traceTransaction" || req.Method == "debug_traceBlockByNumber" {
			result = map[string]interface{}{"trace": "left"}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srvA.Close()

	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := map[string]interface{}{"same": true}
		if req.Method == "debug_traceTransaction" || req.Method == "debug_traceBlockByNumber" {
			result = map[string]interface{}{"trace": "right"}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srvB.Close()

	ds := &dataset.Dataset{
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	cfg := testConfig(srvA.URL, srvB.URL, diff.DefaultOptions())
	cfg.TraceTransaction = true
	cfg.TraceBlock = true
	result, err := replay.Run(context.Background(), ds, cfg, 2, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	traceDiffs := 0
	for _, d := range result.Diffs {
		if d.Category == replay.CategoryTrace {
			traceDiffs++
			if d.Method != "debug_traceTransaction" && d.Method != "debug_traceBlockByNumber" {
				t.Fatalf("unexpected trace diff method: %s", d.Method)
			}
		}
	}
	if traceDiffs != 2 {
		t.Fatalf("expected 2 trace diffs (tx + block), got %d; diffs=%v", traceDiffs, result.Diffs)
	}
}

func TestRun_TraceToggles(t *testing.T) {
	srv := makeEchoServer(t, "0x1")
	ds := &dataset.Dataset{
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	tests := []struct {
		name       string
		traceTx    bool
		traceBlock bool
		wantTotal  int
	}{
		{name: "default_off", wantTotal: 3},
		{name: "trace_tx_only", traceTx: true, wantTotal: 4},
		{name: "trace_block_only", traceBlock: true, wantTotal: 4},
		{name: "trace_both", traceTx: true, traceBlock: true, wantTotal: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(srv.URL, srv.URL, diff.DefaultOptions())
			cfg.TraceTransaction = tc.traceTx
			cfg.TraceBlock = tc.traceBlock

			result, err := replay.Run(context.Background(), ds, cfg, 2, nil)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.TotalRequests != tc.wantTotal {
				t.Fatalf("expected %d total requests, got %d", tc.wantTotal, result.TotalRequests)
			}
		})
	}
}

func TestRun_OnlyTransactionTargets(t *testing.T) {
	srv := makeEchoServer(t, "0x1")
	ds := &dataset.Dataset{
		Accounts:     []dataset.Account{{Address: "0xabc", TxCount: 1}},
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	cfg := testConfig(srv.URL, srv.URL, diff.DefaultOptions())
	cfg.Only = map[string]bool{
		"transaction_by_hash": true,
		"transaction_receipt": true,
	}
	result, err := replay.Run(context.Background(), ds, cfg, 2, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TotalRequests != 2 {
		t.Fatalf("expected 2 transaction-only requests, got %d", result.TotalRequests)
	}
	if result.AccountsTested != 0 || result.BlocksTested != 0 || result.TransactionsTested != 1 {
		t.Fatalf("unexpected tested counters: %+v", result)
	}
}

func TestRun_OnlyTraceTransaction(t *testing.T) {
	srv := makeEchoServer(t, map[string]interface{}{"ok": true})
	ds := &dataset.Dataset{
		Accounts:     []dataset.Account{{Address: "0xabc", TxCount: 1}},
		Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 10, From: "0xabc", To: "0xdef"}},
		Blocks:       []dataset.Block{{Number: 10, TxCount: 1}},
	}

	cfg := testConfig(srv.URL, srv.URL, diff.DefaultOptions())
	cfg.Only = map[string]bool{"trace_transaction": true}
	result, err := replay.Run(context.Background(), ds, cfg, 2, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TotalRequests != 1 {
		t.Fatalf("expected 1 trace-transaction request, got %d", result.TotalRequests)
	}
	if result.AccountsTested != 0 || result.BlocksTested != 0 || result.TransactionsTested != 1 {
		t.Fatalf("unexpected tested counters: %+v", result)
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

func TestRun_Progress(t *testing.T) {
	// Run with a progress writer and verify at least one progress line is emitted.
	srv := makeEchoServer(t, "0x1")

	ds := &dataset.Dataset{
		Accounts: []dataset.Account{{Address: "0xabc", TxCount: 1}},
	}

	opts := diff.DefaultOptions()
	var progBuf bytes.Buffer
	_, err := replay.Run(context.Background(), ds, testConfig(srv.URL, srv.URL, opts), 4, &progBuf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := progBuf.String()
	if !strings.Contains(out, "Progress:") {
		t.Errorf("expected at least one progress line, got: %q", out)
	}
	if !strings.Contains(out, "100.0%") {
		t.Errorf("expected final 100%% progress line, got: %q", out)
	}
}

func TestWriteResultCSV(t *testing.T) {
	r := &replay.Result{
		Diffs: []replay.FoundDiff{
			{Category: replay.CategoryBalance, Method: "eth_getBalance", Params: []interface{}{"0xabc", "0xa"}, Detail: "value mismatch"},
			{Category: replay.CategoryTx, Method: "eth_getTransactionByHash", Params: []interface{}{"0xtx1"}, Detail: "missing"},
		},
	}
	var buf bytes.Buffer
	if err := replay.WriteResultCSV(&buf, r); err != nil {
		t.Fatalf("WriteResultCSV: %v", err)
	}
	out := buf.String()
	// Must have a header row.
	if !strings.Contains(out, "category,method,params,detail") {
		t.Errorf("missing CSV header, got:\n%s", out)
	}
	// Must contain both diff rows.
	if !strings.Contains(out, "balance_mismatch") {
		t.Errorf("expected balance_mismatch row in CSV, got:\n%s", out)
	}
	if !strings.Contains(out, "tx_mismatch") {
		t.Errorf("expected tx_mismatch row in CSV, got:\n%s", out)
	}
	// Verify line count: 1 header + 2 data = 3 lines (plus possible trailing newline).
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 CSV lines (header+2 diffs), got %d:\n%s", len(lines), out)
	}
}

func TestWriteResultCSV_Empty(t *testing.T) {
	r := &replay.Result{}
	var buf bytes.Buffer
	if err := replay.WriteResultCSV(&buf, r); err != nil {
		t.Fatalf("WriteResultCSV: %v", err)
	}
	// Should still have a header row.
	out := strings.TrimSpace(buf.String())
	if out != "category,method,params,detail" {
		t.Errorf("expected only header for empty result, got: %q", out)
	}
}
