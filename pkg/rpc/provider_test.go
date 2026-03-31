package rpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

func TestResolverResolveAliasAndURL(t *testing.T) {
	resolver, err := rpc.NewResolver([]string{"local=https://rpc.example"})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	target, err := resolver.Resolve("local")
	if err != nil {
		t.Fatalf("Resolve(alias) error = %v", err)
	}
	if target.Name != "local" || target.URL != "https://rpc.example" {
		t.Fatalf("Resolve(alias) = %+v", target)
	}

	target, err = resolver.Resolve("https://another.example")
	if err != nil {
		t.Fatalf("Resolve(url) error = %v", err)
	}
	if target.Name != "another.example" {
		t.Fatalf("Resolve(url) name = %q, want host", target.Name)
	}
}

func TestProviderCallRetriesRetryableHTTPStatuses(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"result":  "0x10",
		})
	}))
	defer server.Close()

	provider := rpc.NewProvider(
		rpc.Target{Name: "test", URL: server.URL},
		5*time.Second,
		rpc.WithRetryPolicy(rpc.RetryPolicy{
			MaxAttempts: 3,
			BaseBackoff: time.Millisecond,
			MaxBackoff:  2 * time.Millisecond,
		}),
	)

	response, meta, err := provider.Call(context.Background(), "eth_blockNumber")
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if string(response.Result) != `"0x10"` {
		t.Fatalf("Call() result = %s", response.Result)
	}
	if meta.Attempts != 3 {
		t.Fatalf("Call() attempts = %d, want 3", meta.Attempts)
	}
}

func TestProviderCallDoesNotRetryNonIdempotentMethods(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "temporary", http.StatusBadGateway)
	}))
	defer server.Close()

	provider := rpc.NewProvider(
		rpc.Target{Name: "test", URL: server.URL},
		5*time.Second,
		rpc.WithRetryPolicy(rpc.RetryPolicy{
			MaxAttempts: 4,
			BaseBackoff: time.Millisecond,
			MaxBackoff:  2 * time.Millisecond,
		}),
	)

	_, meta, err := provider.Call(context.Background(), "eth_sendRawTransaction", "0xdeadbeef")
	if err == nil {
		t.Fatal("Call() error = nil, want HTTP error")
	}
	if meta.Attempts != 1 {
		t.Fatalf("Call() attempts = %d, want 1", meta.Attempts)
	}
	if attempts != 1 {
		t.Fatalf("server attempts = %d, want 1", attempts)
	}
}

func TestProviderCallReturnsRPCErrorWithoutRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]any{
				"code":    -32000,
				"message": "state unavailable",
			},
		})
	}))
	defer server.Close()

	provider := rpc.NewProvider(
		rpc.Target{Name: "test", URL: server.URL},
		5*time.Second,
		rpc.WithRetryPolicy(rpc.RetryPolicy{
			MaxAttempts: 4,
			BaseBackoff: time.Millisecond,
			MaxBackoff:  2 * time.Millisecond,
		}),
	)

	_, meta, err := provider.Call(context.Background(), "eth_getBalance", "0xabc", "latest")
	if err == nil {
		t.Fatal("Call() error = nil, want RPC error")
	}

	var rpcErr *rpc.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("Call() error = %T, want *rpc.RPCError", err)
	}
	if meta.Attempts != 1 {
		t.Fatalf("Call() attempts = %d, want 1", meta.Attempts)
	}
	if attempts != 1 {
		t.Fatalf("server attempts = %d, want 1", attempts)
	}
}

func TestProviderCallTimeoutBoundsWholeRetryWindow(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x1",
		})
	}))
	defer server.Close()

	provider := rpc.NewProvider(
		rpc.Target{Name: "test", URL: server.URL},
		50*time.Millisecond,
		rpc.WithRetryPolicy(rpc.RetryPolicy{
			MaxAttempts: 4,
			BaseBackoff: time.Millisecond,
			MaxBackoff:  2 * time.Millisecond,
		}),
	)

	start := time.Now()
	_, meta, err := provider.Call(context.Background(), "eth_getBalance", "0xabc", "latest")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Call() error = nil, want timeout")
	}
	if meta.Attempts != 1 {
		t.Fatalf("Call() attempts = %d, want 1 because timeout should bound retries", meta.Attempts)
	}
	if attempts != 1 {
		t.Fatalf("server attempts = %d, want 1", attempts)
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("Call() elapsed = %v, want bounded by overall timeout", elapsed)
	}
}

func TestParseQuantityUint64(t *testing.T) {
	value, err := rpc.ParseQuantityUint64("0x01")
	if err != nil {
		t.Fatalf("ParseQuantityUint64() error = %v", err)
	}
	if value != 1 {
		t.Fatalf("ParseQuantityUint64() = %d, want 1", value)
	}
}
