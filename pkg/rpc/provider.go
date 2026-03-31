package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const jsonRPCVersion = "2.0"

type Target struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Resolver struct {
	aliases map[string]string
}

func NewResolver(definitions []string) (*Resolver, error) {
	aliases := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		parts := strings.SplitN(definition, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid provider mapping %q: want alias=url", definition)
		}

		alias := strings.TrimSpace(parts[0])
		rawURL := strings.TrimSpace(parts[1])
		if alias == "" || rawURL == "" {
			return nil, fmt.Errorf("invalid provider mapping %q: alias and url are required", definition)
		}
		if !looksLikeURL(rawURL) {
			return nil, fmt.Errorf("invalid provider mapping %q: URL must start with http:// or https://", definition)
		}
		aliases[alias] = rawURL
	}

	return &Resolver{aliases: aliases}, nil
}

func (r *Resolver) Resolve(raw string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, fmt.Errorf("provider target is required")
	}

	if looksLikeURL(raw) {
		return Target{Name: targetNameFromURL(raw), URL: raw}, nil
	}

	if url, ok := r.aliases[raw]; ok {
		return Target{Name: raw, URL: url}, nil
	}

	return Target{}, fmt.Errorf("unknown provider target %q", raw)
}

type RetryPolicy struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 4,
		BaseBackoff: 200 * time.Millisecond,
		MaxBackoff:  2 * time.Second,
	}
}

type Option func(*Provider)

func WithRetryPolicy(policy RetryPolicy) Option {
	return func(p *Provider) {
		p.retry = policy
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) {
		p.client = client
	}
}

func WithUserAgent(userAgent string) Option {
	return func(p *Provider) {
		p.userAgent = userAgent
	}
}

type Provider struct {
	target    Target
	client    *http.Client
	retry     RetryPolicy
	timeout   time.Duration
	userAgent string
	nextID    atomic.Int64
}

func NewProvider(target Target, timeout time.Duration, options ...Option) *Provider {
	transport := &http.Transport{
		MaxIdleConns:        128,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	provider := &Provider{
		target: target,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		retry:     DefaultRetryPolicy(),
		timeout:   timeout,
		userAgent: "rpcduel/0.1",
	}

	for _, option := range options {
		option(provider)
	}

	return provider
}

func (p *Provider) Target() Target {
	return p.target
}

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int64  `json:"id"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("http status %d", e.StatusCode)
	}
	return fmt.Sprintf("http status %d: %s", e.StatusCode, e.Body)
}

type CallMeta struct {
	Target     Target        `json:"target"`
	Attempts   int           `json:"attempts"`
	StatusCode int           `json:"status_code,omitempty"`
	Duration   time.Duration `json:"duration"`
}

func (p *Provider) Call(ctx context.Context, method string, params ...any) (*Response, CallMeta, error) {
	if p.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	payload, err := json.Marshal(Request{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  params,
		ID:      p.nextID.Add(1),
	})
	if err != nil {
		return nil, CallMeta{Target: p.target}, fmt.Errorf("marshal JSON-RPC request: %w", err)
	}

	start := time.Now()
	meta := CallMeta{Target: p.target}
	attempts := 1
	if isRetrySafeMethod(method) {
		attempts = max(1, p.retry.MaxAttempts)
	}
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		meta.Attempts = attempt

		request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.target.URL, bytes.NewReader(payload))
		if err != nil {
			meta.Duration = time.Since(start)
			return nil, meta, fmt.Errorf("create HTTP request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		if p.userAgent != "" {
			request.Header.Set("User-Agent", p.userAgent)
		}

		response, err := p.client.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("perform JSON-RPC request: %w", err)
			meta.Duration = time.Since(start)
			if attempt == attempts || !shouldRetryTransportError(ctx, err) || !sleepBackoff(ctx, p.retry, attempt) {
				return nil, meta, lastErr
			}
			continue
		}

		statusCode := response.StatusCode
		meta.StatusCode = statusCode
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read JSON-RPC response: %w", readErr)
			meta.Duration = time.Since(start)
			if attempt == attempts || !sleepBackoff(ctx, p.retry, attempt) {
				return nil, meta, lastErr
			}
			continue
		}

		if statusCode != http.StatusOK {
			lastErr = &HTTPStatusError{
				StatusCode: statusCode,
				Body:       strings.TrimSpace(string(body)),
			}
			meta.Duration = time.Since(start)
			if attempt == attempts || !isRetryableStatus(statusCode) || !sleepBackoff(ctx, p.retry, attempt) {
				return nil, meta, lastErr
			}
			continue
		}

		var rpcResponse Response
		if err := json.Unmarshal(body, &rpcResponse); err != nil {
			meta.Duration = time.Since(start)
			return nil, meta, fmt.Errorf("decode JSON-RPC response: %w", err)
		}

		meta.Duration = time.Since(start)
		if rpcResponse.Error != nil {
			return &rpcResponse, meta, rpcResponse.Error
		}
		return &rpcResponse, meta, nil
	}

	meta.Duration = time.Since(start)
	if lastErr == nil {
		lastErr = fmt.Errorf("JSON-RPC call failed without a concrete error")
	}
	return nil, meta, lastErr
}

func (p *Provider) BlockByNumber(ctx context.Context, number uint64, fullTransactions bool) (*Response, CallMeta, error) {
	return p.Call(ctx, "eth_getBlockByNumber", HexBlockNumber(number), fullTransactions)
}

func (p *Provider) BlockByTag(ctx context.Context, tag string, fullTransactions bool) (*Response, CallMeta, error) {
	return p.Call(ctx, "eth_getBlockByNumber", tag, fullTransactions)
}

func (p *Provider) TransactionByHash(ctx context.Context, hash string) (*Response, CallMeta, error) {
	return p.Call(ctx, "eth_getTransactionByHash", hash)
}

func (p *Provider) ReceiptByHash(ctx context.Context, hash string) (*Response, CallMeta, error) {
	return p.Call(ctx, "eth_getTransactionReceipt", hash)
}

func (p *Provider) BalanceAt(ctx context.Context, address, blockTag string) (*Response, CallMeta, error) {
	return p.Call(ctx, "eth_getBalance", address, blockTag)
}

func (p *Provider) NonceAt(ctx context.Context, address, blockTag string) (*Response, CallMeta, error) {
	return p.Call(ctx, "eth_getTransactionCount", address, blockTag)
}

func HexBlockNumber(number uint64) string {
	return fmt.Sprintf("0x%x", number)
}

func PreviousBlockTag(number uint64) (string, bool) {
	if number == 0 {
		return "", false
	}
	return HexBlockNumber(number - 1), true
}

func ParseQuantityBig(value string) (*big.Int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("empty quantity")
	}

	if strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
		number := new(big.Int)
		if _, ok := number.SetString(trimmed[2:], 16); ok {
			return number, nil
		}
		return nil, fmt.Errorf("invalid hex quantity %q", value)
	}

	number := new(big.Int)
	if _, ok := number.SetString(trimmed, 10); ok {
		return number, nil
	}
	return nil, fmt.Errorf("invalid decimal quantity %q", value)
}

func ParseQuantityUint64(value string) (uint64, error) {
	number, err := ParseQuantityBig(value)
	if err != nil {
		return 0, err
	}
	if !number.IsUint64() {
		return 0, fmt.Errorf("quantity %q does not fit in uint64", value)
	}
	return number.Uint64(), nil
}

func StatusCodeOf(err error) int {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode
	}
	return 0
}

func looksLikeURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

func targetNameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	return parsed.Host
}

func shouldRetryTransportError(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	return true
}

func isRetrySafeMethod(method string) bool {
	switch {
	case strings.HasPrefix(method, "eth_get"),
		strings.HasPrefix(method, "net_"),
		strings.HasPrefix(method, "web3_"),
		strings.HasPrefix(method, "trace_"),
		strings.HasPrefix(method, "debug_trace"):
		return true
	}

	switch method {
	case "eth_blockNumber",
		"eth_call",
		"eth_chainId",
		"eth_estimateGas",
		"eth_feeHistory",
		"eth_gasPrice",
		"eth_maxPriorityFeePerGas",
		"eth_protocolVersion",
		"eth_syncing":
		return true
	default:
		return false
	}
}

func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func sleepBackoff(ctx context.Context, policy RetryPolicy, attempt int) bool {
	backoff := policy.BaseBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}

	maxBackoff := policy.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = backoff
	}

	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= maxBackoff {
			backoff = maxBackoff
			break
		}
	}

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
