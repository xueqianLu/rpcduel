package dataset_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/dataset"
)

// mockBlockscout sets up a test Blockscout server returning a single page of items.
func mockBlockscoutAddresses(t *testing.T, items []map[string]interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		itemsJSON, _ := json.Marshal(items)
		resp := map[string]interface{}{
			"items":            json.RawMessage(itemsJSON),
			"next_page_params": nil,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mockBlockscoutTxs(t *testing.T, items []map[string]interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		itemsJSON, _ := json.Marshal(items)
		resp := map[string]interface{}{
			"items":            json.RawMessage(itemsJSON),
			"next_page_params": nil,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchAccounts(t *testing.T) {
	items := []map[string]interface{}{
		{"hash": "0xaaa", "transactions_count": "100"},
		{"hash": "0xbbb", "transactions_count": "50"},
	}
	srv := mockBlockscoutAddresses(t, items)
	c := dataset.NewBlockscoutClient(srv.URL, 100)
	accounts, err := c.FetchAccounts(context.Background(), 10)
	if err != nil {
		t.Fatalf("FetchAccounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(accounts))
	}
	if accounts[0].Address != "0xaaa" {
		t.Errorf("expected 0xaaa first, got %s", accounts[0].Address)
	}
}

func TestFetchAccounts_Limit(t *testing.T) {
	items := []map[string]interface{}{
		{"hash": "0xaaa", "transactions_count": "100"},
		{"hash": "0xbbb", "transactions_count": "50"},
		{"hash": "0xccc", "transactions_count": "30"},
	}
	srv := mockBlockscoutAddresses(t, items)
	c := dataset.NewBlockscoutClient(srv.URL, 100)
	accounts, err := c.FetchAccounts(context.Background(), 2)
	if err != nil {
		t.Fatalf("FetchAccounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("expected limit of 2 accounts, got %d", len(accounts))
	}
}

func TestFetchTransactions_BlockFilter(t *testing.T) {
	items := []map[string]interface{}{
		{
			"hash":         "0xtx1",
			"block_number": 150,
			"from":         map[string]string{"hash": "0xabc"},
			"to":           map[string]string{"hash": "0xdef"},
			"status":       "ok",
		},
		{
			"hash":         "0xtx2",
			"block_number": 50, // outside range
			"from":         map[string]string{"hash": "0xabc"},
			"to":           map[string]string{"hash": "0xdef"},
			"status":       "ok",
		},
	}
	srv := mockBlockscoutTxs(t, items)
	c := dataset.NewBlockscoutClient(srv.URL, 100)
	txs, err := c.FetchTransactions(context.Background(), 100, 200, 10)
	if err != nil {
		t.Fatalf("FetchTransactions: %v", err)
	}
	// Only tx1 is in range [100, 200]
	if len(txs) != 1 {
		t.Errorf("expected 1 tx in range, got %d", len(txs))
	}
	if len(txs) > 0 && txs[0].Hash != "0xtx1" {
		t.Errorf("expected 0xtx1, got %s", txs[0].Hash)
	}
}

func TestFetchBlocks(t *testing.T) {
	items := []map[string]interface{}{
		{"height": 150, "transaction_count": 3},
		{"height": 200, "transaction_count": 0}, // no txs, should be filtered
		{"height": 50, "transaction_count": 5},  // out of range
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		itemsJSON, _ := json.Marshal(items)
		resp := map[string]interface{}{
			"items":            json.RawMessage(itemsJSON),
			"next_page_params": nil,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := dataset.NewBlockscoutClient(srv.URL, 100)
	blocks, err := c.FetchBlocks(context.Background(), 100, 200, 10)
	if err != nil {
		t.Fatalf("FetchBlocks: %v", err)
	}
	// Only block 150 is in range and has txs
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d: %+v", len(blocks), blocks)
	}
}
