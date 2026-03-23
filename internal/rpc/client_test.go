package rpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

func newTestServer(t *testing.T, result interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req rpc.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

func TestClient_Call_Success(t *testing.T) {
	srv := newTestServer(t, "0x1")
	c := rpc.NewClient(srv.URL, 5*time.Second)
	resp, lat, err := c.Call(context.Background(), "eth_blockNumber", []interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lat == 0 {
		t.Error("expected non-zero latency")
	}
	if string(resp.Result) != `"0x1"` {
		t.Errorf("unexpected result: %s", resp.Result)
	}
}

func TestClient_Call_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := rpc.NewClient(srv.URL, 5*time.Second)
	_, _, err := c.Call(context.Background(), "eth_blockNumber", []interface{}{})
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestParseParams_Empty(t *testing.T) {
	params, err := rpc.ParseParams("[]")
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 0 {
		t.Errorf("expected empty params, got %v", params)
	}
}

func TestParseParams_Array(t *testing.T) {
	params, err := rpc.ParseParams(`["latest", false]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}
}

func TestParseParams_Invalid(t *testing.T) {
	_, err := rpc.ParseParams(`not json`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
