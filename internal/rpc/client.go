// Package rpc provides a JSON-RPC HTTP client for Ethereum nodes.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// Options controls the behavior of a Client beyond the basics.
type Options struct {
	// Timeout is the per-attempt HTTP request timeout. Required.
	Timeout time.Duration
	// Retries is the maximum number of additional attempts after the first
	// failure. Zero disables retries.
	Retries int
	// RetryBackoff is the base backoff between retries. Each retry waits
	// RetryBackoff * 2^(attempt-1).
	RetryBackoff time.Duration
	// Headers are extra HTTP headers added to every request.
	Headers map[string]string
	// UserAgent overrides the default User-Agent header. If empty, the
	// default Go HTTP client User-Agent is sent.
	UserAgent string
}

// Client is an Ethereum JSON-RPC HTTP client.
type Client struct {
	endpoint string
	http     *http.Client
	opts     Options
}

// NewClient creates a new RPC client for the given endpoint with default
// options aside from the timeout.
func NewClient(endpoint string, timeout time.Duration) *Client {
	return NewClientWithOptions(endpoint, Options{Timeout: timeout})
}

// NewClientWithOptions creates a new RPC client with the given options.
func NewClientWithOptions(endpoint string, opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Retries < 0 {
		opts.Retries = 0
	}
	if opts.RetryBackoff <= 0 {
		opts.RetryBackoff = 200 * time.Millisecond
	}
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
			Timeout:   opts.Timeout,
		},
		opts: opts,
	}
}

// Endpoint returns the configured RPC endpoint URL.
func (c *Client) Endpoint() string { return c.endpoint }

// Call sends a JSON-RPC request and returns the response. The returned
// duration covers all attempts (including any retries).
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
	var (
		resp    *Response
		lastErr error
	)
	for attempt := 0; attempt <= c.opts.Retries; attempt++ {
		if attempt > 0 {
			wait := c.opts.RetryBackoff << (attempt - 1)
			select {
			case <-ctx.Done():
				return nil, time.Since(start), ctx.Err()
			case <-time.After(wait):
			}
		}
		resp, lastErr = c.doOnce(ctx, body)
		if !shouldRetry(lastErr) {
			break
		}
	}
	return resp, time.Since(start), lastErr
}

func (c *Client) doOnce(ctx context.Context, body []byte) (*Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.opts.UserAgent != "" {
		httpReq.Header.Set("User-Agent", c.opts.UserAgent)
	}
	for k, v := range c.opts.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{Status: resp.StatusCode}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var rpcResp Response
	if err := json.Unmarshal(data, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if rpcResp.Error != nil {
		return &rpcResp, rpcResp.Error
	}
	return &rpcResp, nil
}

// httpStatusError is returned for non-200 HTTP responses so callers (and the
// retry logic) can distinguish them from other transport errors.
type httpStatusError struct{ Status int }

func (e *httpStatusError) Error() string { return fmt.Sprintf("http status %d", e.Status) }

// shouldRetry returns true for transient errors that are worth retrying.
// JSON-RPC application errors (RPCError) are NOT retried because they
// represent a deterministic response from the node.
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) {
		return false
	}
	var httpErr *httpStatusError
	if errors.As(err, &httpErr) {
		// Retry on 408 Request Timeout, 429 Too Many Requests, and 5xx.
		return httpErr.Status == http.StatusRequestTimeout ||
			httpErr.Status == http.StatusTooManyRequests ||
			httpErr.Status >= 500
	}
	// Network-level / transport errors.
	return true
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

// ParsePositionalParams parses CLI positional args into JSON-RPC params.
//
// Each token is interpreted as JSON when possible (for example: true, false,
// null, 123, {"k":1}, [1,2]). Tokens that are not valid standalone JSON,
// such as latest, 0xa11111, addresses, and hashes, are kept as plain strings.
func ParsePositionalParams(args []string) ([]interface{}, error) {
	params := make([]interface{}, 0, len(args))
	for _, arg := range args {
		params = append(params, parsePositionalArg(arg))
	}
	return params, nil
}

func parsePositionalArg(raw string) interface{} {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()

	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return raw
	}

	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return raw
	}

	return v
}

