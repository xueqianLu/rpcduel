// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

func TestClient_RetriesOn5xx(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
	}))
	defer srv.Close()

	c := rpc.NewClientWithOptions(srv.URL, rpc.Options{
		Timeout:      2 * time.Second,
		Retries:      4,
		RetryBackoff: 1 * time.Millisecond,
	})
	resp, _, err := c.Call(context.Background(), "eth_blockNumber", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(resp.Result) != `"0x1"` {
		t.Fatalf("unexpected result: %s", resp.Result)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestClient_DoesNotRetryRPCError(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`))
	}))
	defer srv.Close()

	c := rpc.NewClientWithOptions(srv.URL, rpc.Options{
		Timeout:      2 * time.Second,
		Retries:      3,
		RetryBackoff: 1 * time.Millisecond,
	})
	_, _, err := c.Call(context.Background(), "eth_blockNumber", nil)
	if err == nil {
		t.Fatal("expected RPC error")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
}

func TestClient_GivesUpAfterRetries(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := rpc.NewClientWithOptions(srv.URL, rpc.Options{
		Timeout:      2 * time.Second,
		Retries:      2,
		RetryBackoff: 1 * time.Millisecond,
	})
	_, _, err := c.Call(context.Background(), "eth_blockNumber", nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", got)
	}
}

func TestClient_AppliesHeadersAndUserAgent(t *testing.T) {
	gotHeaders := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
	}))
	defer srv.Close()

	c := rpc.NewClientWithOptions(srv.URL, rpc.Options{
		Timeout: 2 * time.Second,
		Headers: map[string]string{
			"X-Api-Key":  "secret",
			"X-Trace-Id": "abc",
		},
		UserAgent: "rpcduel-test/1.0",
	})
	if _, _, err := c.Call(context.Background(), "eth_blockNumber", nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
	h := <-gotHeaders
	if got := h.Get("X-Api-Key"); got != "secret" {
		t.Errorf("X-Api-Key=%q", got)
	}
	if got := h.Get("X-Trace-Id"); got != "abc" {
		t.Errorf("X-Trace-Id=%q", got)
	}
	if got := h.Get("User-Agent"); got != "rpcduel-test/1.0" {
		t.Errorf("User-Agent=%q", got)
	}
}

func TestClient_Endpoint(t *testing.T) {
	c := rpc.NewClient("https://example.com", time.Second)
	if got := c.Endpoint(); got != "https://example.com" {
		t.Fatalf("Endpoint=%q", got)
	}
}

func TestClient_CustomTransportIsUsed(t *testing.T) {
	var seen int32
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&seen, 1)
		body := `{"jsonrpc":"2.0","id":1,"result":"0x1"}`
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	c := rpc.NewClientWithOptions("http://nowhere.invalid", rpc.Options{
		Timeout:   time.Second,
		Transport: rt,
	})
	if _, _, err := c.Call(context.Background(), "eth_blockNumber", nil); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if atomic.LoadInt32(&seen) != 1 {
		t.Fatalf("custom transport was not used (seen=%d)", seen)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
