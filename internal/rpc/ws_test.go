package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestIsWebSocketEndpoint(t *testing.T) {
	cases := map[string]bool{
		"ws://localhost:8546":    true,
		"wss://node.example/rpc": true,
		"http://localhost:8545":  false,
		"https://node.example":   false,
		"unix:///tmp/geth.ipc":   false,
		"":                       false,
		"://broken":              false,
	}
	for in, want := range cases {
		if got := IsWebSocketEndpoint(in); got != want {
			t.Errorf("IsWebSocketEndpoint(%q) = %v, want %v", in, got, want)
		}
	}
}

// echoUpgrader returns an httptest.Server that upgrades to a WebSocket and
// echoes JSON-RPC requests back as responses with the requested method
// embedded as the result.
func echoUpgrader(t *testing.T, calls *int64) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer c.Close()
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			var req Request
			if err := json.Unmarshal(data, &req); err != nil {
				return
			}
			atomic.AddInt64(calls, 1)
			result, _ := json.Marshal(req.Method)
			resp := Response{JSONRPC: "2.0", ID: req.ID, Result: result}
			out, _ := json.Marshal(resp)
			if err := c.WriteMessage(websocket.TextMessage, out); err != nil {
				return
			}
		}
	}))
}

func TestWSClient_BasicCall(t *testing.T) {
	var calls int64
	srv := echoUpgrader(t, &calls)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := NewClient(wsURL, 5*time.Second)
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

func TestWSClient_Concurrent(t *testing.T) {
	var calls int64
	srv := echoUpgrader(t, &calls)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := NewClient(wsURL, 5*time.Second)
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

func TestWSClient_ContextCancel(t *testing.T) {
	// Server that accepts upgrade but never replies.
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		// Hold the connection open without writing anything until the
		// peer disconnects.
		for {
			if _, _, err := c.NextReader(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c := NewClient(wsURL, 5*time.Second)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := c.Call(ctx, "eth_blockNumber", nil)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}
