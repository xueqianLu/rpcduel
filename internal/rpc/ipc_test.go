// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsIPCEndpoint(t *testing.T) {
	cases := map[string]bool{
		"unix:///tmp/geth.ipc":   true,
		"unix:///var/run/x.sock": true,
		"http://localhost:8545":  false,
		"ws://localhost:8546":    false,
		"":                       false,
	}
	for in, want := range cases {
		if got := IsIPCEndpoint(in); got != want {
			t.Errorf("IsIPCEndpoint(%q) = %v, want %v", in, got, want)
		}
	}
}

// startEchoIPC listens on a temp unix socket and echoes JSON-RPC requests
// back as responses where the result equals the requested method name.
func startEchoIPC(t *testing.T, calls *int64) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "rpc.ipc")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				dec := json.NewDecoder(c)
				enc := json.NewEncoder(c)
				for {
					var req Request
					if err := dec.Decode(&req); err != nil {
						return
					}
					atomic.AddInt64(calls, 1)
					result, _ := json.Marshal(req.Method)
					if err := enc.Encode(Response{JSONRPC: "2.0", ID: req.ID, Result: result}); err != nil {
						return
					}
				}
			}(c)
		}
	}()
	return "unix://" + sock, func() { _ = ln.Close() }
}

func TestIPCClient_BasicCall(t *testing.T) {
	var calls int64
	endpoint, cleanup := startEchoIPC(t, &calls)
	defer cleanup()

	c := NewClient(endpoint, 5*time.Second)
	defer c.Close()

	resp, _, err := c.Call(context.Background(), "eth_blockNumber", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var got string
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != "eth_blockNumber" {
		t.Errorf("got %q want eth_blockNumber", got)
	}
	if atomic.LoadInt64(&calls) != 1 {
		t.Errorf("server saw %d calls, want 1", calls)
	}
}

func TestIPCClient_Concurrent(t *testing.T) {
	var calls int64
	endpoint, cleanup := startEchoIPC(t, &calls)
	defer cleanup()

	c := NewClient(endpoint, 5*time.Second)
	defer c.Close()

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if _, _, err := c.Call(context.Background(), "eth_chainId", nil); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("call err: %v", err)
	}
	if got := atomic.LoadInt64(&calls); got != N {
		t.Errorf("server saw %d calls, want %d", got, N)
	}
}

func TestIPCClient_DialError(t *testing.T) {
	c := NewClient("unix:///nonexistent/no.sock", 500*time.Millisecond)
	defer c.Close()
	_, _, err := c.Call(context.Background(), "eth_blockNumber", nil)
	if err == nil {
		t.Fatal("expected dial error, got nil")
	}
}
