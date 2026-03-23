// Package rpc provides a JSON-RPC HTTP client for Ethereum nodes.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int64         `json:"id"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

var idCounter int64

// Client is an Ethereum JSON-RPC HTTP client.
type Client struct {
	endpoint string
	http     *http.Client
}

// NewClient creates a new RPC client for the given endpoint.
func NewClient(endpoint string, timeout time.Duration) *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}
	return &Client{
		endpoint: endpoint,
		http: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// Call sends a JSON-RPC request and returns the response.
func (c *Client) Call(ctx context.Context, method string, params []interface{}) (*Response, time.Duration, error) {
	id := atomic.AddInt64(&idCounter, 1)
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal request: %w", err)
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	elapsed := time.Since(start)
	if err != nil {
		return nil, elapsed, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, elapsed, fmt.Errorf("http status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, elapsed, fmt.Errorf("read body: %w", err)
	}

	var rpcResp Response
	if err := json.Unmarshal(data, &rpcResp); err != nil {
		return nil, elapsed, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return &rpcResp, elapsed, rpcResp.Error
	}

	return &rpcResp, elapsed, nil
}

// ParseParams parses a JSON string into a slice of interface{}.
func ParseParams(raw string) ([]interface{}, error) {
	if raw == "" || raw == "[]" {
		return []interface{}{}, nil
	}
	var params []interface{}
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	return params, nil
}
