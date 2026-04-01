package rpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestParsePositionalParams_PlainStrings(t *testing.T) {
	params, err := rpc.ParsePositionalParams([]string{"0xa11111", "latest"})
	if err != nil {
		t.Fatalf("ParsePositionalParams: %v", err)
	}
	want := []interface{}{"0xa11111", "latest"}
	if !reflect.DeepEqual(params, want) {
		t.Fatalf("expected %v, got %v", want, params)
	}
}

func TestParsePositionalParams_JSONLiterals(t *testing.T) {
	params, err := rpc.ParsePositionalParams([]string{"true", "null", "123", `{"k":1}`, `[1,2]`})
	if err != nil {
		t.Fatalf("ParsePositionalParams: %v", err)
	}

	if got, ok := params[0].(bool); !ok || !got {
		t.Fatalf("expected bool true, got %#v", params[0])
	}
	if params[1] != nil {
		t.Fatalf("expected nil, got %#v", params[1])
	}
	if got, ok := params[2].(json.Number); !ok || got.String() != "123" {
		t.Fatalf("expected json.Number(123), got %#v", params[2])
	}
	obj, ok := params[3].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object param, got %#v", params[3])
	}
	if got, ok := obj["k"].(json.Number); !ok || got.String() != "1" {
		t.Fatalf("expected object field json.Number(1), got %#v", obj["k"])
	}
	arr, ok := params[4].([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("expected array param, got %#v", params[4])
	}
}
