package dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/dataset"
)

// mockRPCServer creates a test JSON-RPC server. The handler function receives
// the method and params from each request and returns the result as raw JSON.
func mockRPCServer(t *testing.T, handler func(method string, params json.RawMessage) interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		result := handler(req.Method, req.Params)
		resultJSON, _ := json.Marshal(result)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(resultJSON),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestChainScanner_LatestBlockNumber(t *testing.T) {
	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method == "eth_blockNumber" {
			return "0x64" // 100 in decimal
		}
		return nil
	})

	scanner := dataset.NewChainScanner(srv.URL)
	num, err := scanner.LatestBlockNumber(context.Background())
	if err != nil {
		t.Fatalf("LatestBlockNumber: %v", err)
	}
	if num != 100 {
		t.Errorf("expected 100, got %d", num)
	}
}

func TestChainScanner_Scan_BasicCollection(t *testing.T) {
	// Serve blocks 110, 109, 108 (high to low). Block 109 has no txs.
	blocks := map[int64]map[string]interface{}{
		110: {
			"number": "0x6e",
			"transactions": []map[string]interface{}{
				{"hash": "0xtx1", "from": "0xaaa", "to": "0xbbb"},
				{"hash": "0xtx2", "from": "0xccc", "to": "0xaaa"},
			},
		},
		109: {
			"number":       "0x6d",
			"transactions": []map[string]interface{}{},
		},
		108: {
			"number": "0x6c",
			"transactions": []map[string]interface{}{
				{"hash": "0xtx3", "from": "0xbbb", "to": "0xddd"},
			},
		},
	}

	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method != "eth_getBlockByNumber" {
			return nil
		}
		var p []json.RawMessage
		_ = json.Unmarshal(params, &p)
		var hexNum string
		_ = json.Unmarshal(p[0], &hexNum)
		// parse hex block number
		var num int64
		fmt.Sscanf(hexNum[2:], "%x", &num)
		b, ok := blocks[num]
		if !ok {
			return nil
		}
		return b
	})

	scanner := dataset.NewChainScanner(srv.URL)
	accounts, txs, blks, err := scanner.Scan(context.Background(), 108, 110, 100, 100, 100, 4)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Expect 2 blocks with transactions (110 and 108), not 109 (empty).
	if len(blks) != 2 {
		t.Errorf("expected 2 blocks, got %d: %+v", len(blks), blks)
	}

	// Expect 3 transactions.
	if len(txs) != 3 {
		t.Errorf("expected 3 transactions, got %d: %+v", len(txs), txs)
	}

	// Unique addresses: 0xaaa, 0xbbb, 0xccc, 0xddd.
	if len(accounts) != 4 {
		t.Errorf("expected 4 accounts, got %d: %+v", len(accounts), accounts)
	}
}

func TestChainScanner_Scan_TxLimit(t *testing.T) {
	// One block with 5 transactions; limit to 3.
	txItems := make([]map[string]interface{}, 5)
	for i := range txItems {
		txItems[i] = map[string]interface{}{
			"hash": fmt.Sprintf("0xtx%d", i),
			"from": fmt.Sprintf("0xfrom%d", i),
			"to":   fmt.Sprintf("0xto%d", i),
		}
	}

	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method != "eth_getBlockByNumber" {
			return nil
		}
		return map[string]interface{}{
			"number":       "0x64",
			"transactions": txItems,
		}
	})

	scanner := dataset.NewChainScanner(srv.URL)
	_, txs, _, err := scanner.Scan(context.Background(), 100, 100, 100, 3, 100, 1)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(txs) != 3 {
		t.Errorf("expected txs capped at 3, got %d", len(txs))
	}
}

func TestChainScanner_Scan_BlocksDescending(t *testing.T) {
	// Blocks 200, 199, 198 – verify we collect them high-to-low order.
	callOrder := []int64{}
	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method != "eth_getBlockByNumber" {
			return nil
		}
		var p []json.RawMessage
		_ = json.Unmarshal(params, &p)
		var hexNum string
		_ = json.Unmarshal(p[0], &hexNum)
		var num int64
		fmt.Sscanf(hexNum[2:], "%x", &num)
		callOrder = append(callOrder, num)
		return map[string]interface{}{
			"number": hexNum,
			"transactions": []map[string]interface{}{
				{"hash": fmt.Sprintf("0xtx%d", num), "from": "0xaaa", "to": "0xbbb"},
			},
		}
	})

	scanner := dataset.NewChainScanner(srv.URL)
	_, _, _, err := scanner.Scan(context.Background(), 198, 200, 100, 100, 100, 1)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 block fetches, got %d", len(callOrder))
	}
	// Must be called in descending order: 200, 199, 198.
	if callOrder[0] != 200 || callOrder[1] != 199 || callOrder[2] != 198 {
		t.Errorf("expected descending order [200 199 198], got %v", callOrder)
	}
}

func TestChainScanner_Scan_AccountsSortedByTxCount(t *testing.T) {
	// 0xhot appears in 3 transactions, 0xcold in 1.
	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method != "eth_getBlockByNumber" {
			return nil
		}
		return map[string]interface{}{
			"number": "0x64",
			"transactions": []map[string]interface{}{
				{"hash": "0xtx1", "from": "0xhot", "to": "0xcold"},
				{"hash": "0xtx2", "from": "0xhot", "to": "0xother"},
				{"hash": "0xtx3", "from": "0xhot", "to": "0xother2"},
			},
		}
	})

	scanner := dataset.NewChainScanner(srv.URL)
	accounts, _, _, err := scanner.Scan(context.Background(), 100, 100, 100, 100, 100, 4)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// Hottest account must come first.
	if len(accounts) == 0 {
		t.Fatal("no accounts returned")
	}
	if accounts[0].Address != "0xhot" {
		t.Errorf("expected 0xhot first (highest tx count), got %s", accounts[0].Address)
	}
}

func TestChainScanner_Scan_NullBlock(t *testing.T) {
	// First call returns null (block doesn't exist), second has real data.
	call := 0
	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method != "eth_getBlockByNumber" {
			return nil
		}
		call++
		if call == 1 {
			return nil // null block
		}
		return map[string]interface{}{
			"number": "0x63",
			"transactions": []map[string]interface{}{
				{"hash": "0xtx1", "from": "0xaaa", "to": "0xbbb"},
			},
		}
	})

	scanner := dataset.NewChainScanner(srv.URL)
	_, txs, blks, err := scanner.Scan(context.Background(), 99, 100, 100, 100, 100, 1)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// Null block should be skipped; one real block should be collected.
	if len(blks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blks))
	}
	if len(txs) != 1 {
		t.Errorf("expected 1 tx, got %d", len(txs))
	}
}

func TestChainScanner_Scan_Concurrent(t *testing.T) {
	// 10 blocks, each with 2 transactions. Verify totals are correct under
	// concurrent fetching (concurrency=4).
	srv := mockRPCServer(t, func(method string, params json.RawMessage) interface{} {
		if method != "eth_getBlockByNumber" {
			return nil
		}
		var p []json.RawMessage
		_ = json.Unmarshal(params, &p)
		var hexNum string
		_ = json.Unmarshal(p[0], &hexNum)
		var num int64
		fmt.Sscanf(hexNum[2:], "%x", &num)
		return map[string]interface{}{
			"number": hexNum,
			"transactions": []map[string]interface{}{
				{"hash": fmt.Sprintf("0xtx%da", num), "from": fmt.Sprintf("0xfrom%d", num), "to": fmt.Sprintf("0xto%d", num)},
				{"hash": fmt.Sprintf("0xtx%db", num), "from": fmt.Sprintf("0xfrom%d", num), "to": fmt.Sprintf("0xto%d", num)},
			},
		}
	})

	scanner := dataset.NewChainScanner(srv.URL)
	accounts, txs, blks, err := scanner.Scan(context.Background(), 1, 10, 1000, 1000, 1000, 4)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(blks) != 10 {
		t.Errorf("expected 10 blocks, got %d", len(blks))
	}
	if len(txs) != 20 {
		t.Errorf("expected 20 transactions, got %d", len(txs))
	}
	// Each block has the same from/to pair → 2 unique accounts per block, but
	// from addr is the same across txs in the same block (deduped).
	// 10 unique from addresses + 10 unique to addresses = 20 unique accounts.
	if len(accounts) != 20 {
		t.Errorf("expected 20 accounts, got %d", len(accounts))
	}
}
