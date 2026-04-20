package runner

import (
	"context"

	"golang.org/x/time/rate"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

type ctxKey int

const (
	ctxKeyClientOptions ctxKey = iota
	ctxKeyRateLimiter
)

// WithClientOptions returns a context that carries rpc.Options. All runner
// loops use these options when constructing rpc.Clients, ensuring CLI flags
// like --retries, --headers, --insecure, and --user-agent are honored.
//
// The Timeout field of opts is overridden per-call by the timeout passed to
// each Run* function.
func WithClientOptions(ctx context.Context, opts rpc.Options) context.Context {
	return context.WithValue(ctx, ctxKeyClientOptions, opts)
}

// ClientOptionsFromContext returns the rpc.Options stored in ctx, or the
// zero value if none was attached.
func ClientOptionsFromContext(ctx context.Context) (rpc.Options, bool) {
	v, ok := ctx.Value(ctxKeyClientOptions).(rpc.Options)
	return v, ok
}

// WithRateLimiter returns a context that carries a token-bucket rate limiter.
// Every runner worker will Wait on the limiter before issuing each RPC call,
// shaping the aggregate request rate across all workers.
func WithRateLimiter(ctx context.Context, lim *rate.Limiter) context.Context {
	if lim == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRateLimiter, lim)
}

// rateLimiterFromContext returns the limiter attached to ctx, or nil.
func rateLimiterFromContext(ctx context.Context) *rate.Limiter {
	if v, ok := ctx.Value(ctxKeyRateLimiter).(*rate.Limiter); ok {
		return v
	}
	return nil
}

// waitRate blocks until the limiter (if any) issues a token, or returns the
// context's error. Returns nil if no limiter is configured.
func waitRate(ctx context.Context) error {
	lim := rateLimiterFromContext(ctx)
	if lim == nil {
		return nil
	}
	return lim.Wait(ctx)
}
