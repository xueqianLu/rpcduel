// Package metrics exposes Prometheus metrics for rpcduel benchmarks.
//
// All collectors are registered on a package-private registry that is also
// served when the user passes --metrics-addr. The Observe and ObserveDiff
// helpers are safe to call unconditionally — they simply update the
// registered collectors and incur no I/O if the exporter HTTP server has
// not been started.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	registry = prometheus.NewRegistry()

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpcduel_requests_total",
			Help: "Total number of RPC requests issued by rpcduel, partitioned by endpoint, scenario, and outcome (ok|error).",
		},
		[]string{"endpoint", "scenario", "status"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpcduel_request_duration_seconds",
			Help:    "RPC request latency in seconds, partitioned by endpoint and scenario.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"endpoint", "scenario"},
	)

	diffsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpcduel_diffs_total",
			Help: "Total number of response differences observed by rpcduel duel runs, partitioned by endpoint pair.",
		},
		[]string{"endpoint_a", "endpoint_b"},
	)

	replayDiffsByCategory = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpcduel_replay_diffs_total",
			Help: "Total number of replay diffs observed, partitioned by category (balance_mismatch, nonce_mismatch, tx_mismatch, receipt_mismatch, trace_mismatch, block_mismatch, missing_data, rpc_error).",
		},
		[]string{"category"},
	)

	startOnce sync.Once
)

func init() {
	registry.MustRegister(
		requestsTotal,
		requestDuration,
		diffsTotal,
		replayDiffsByCategory,
	)
}

// Observe records a single RPC outcome. scenario may be empty.
func Observe(endpoint, scenario string, latency time.Duration, isError bool) {
	status := "ok"
	if isError {
		status = "error"
	}
	requestsTotal.WithLabelValues(endpoint, scenario, status).Inc()
	requestDuration.WithLabelValues(endpoint, scenario).Observe(latency.Seconds())
}

// ObserveDiff increments the differences counter for an endpoint pair.
func ObserveDiff(endpointA, endpointB string, n int) {
	if n <= 0 {
		return
	}
	diffsTotal.WithLabelValues(endpointA, endpointB).Add(float64(n))
}

// ObserveReplayCategory records the number of replay diffs seen in a given category.
func ObserveReplayCategory(category string, n int) {
	if n <= 0 {
		return
	}
	replayDiffsByCategory.WithLabelValues(category).Add(float64(n))
}

// Push sends all collected metrics to a Prometheus Pushgateway at
// gatewayURL under the given job name. Grouping labels are optional.
// A zero-length gatewayURL is a no-op so callers can wire this to a
// flag unconditionally.
func Push(gatewayURL, job string, grouping map[string]string) error {
	if gatewayURL == "" {
		return nil
	}
	p := push.New(gatewayURL, job).Gatherer(registry)
	for k, v := range grouping {
		p = p.Grouping(k, v)
	}
	return p.Push()
}

// Handler returns the HTTP handler that exposes the rpcduel metrics
// registry in the Prometheus text exposition format.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry})
}

// StartServer starts an HTTP server on addr that exposes /metrics.
// It is safe to call multiple times — only the first call has effect.
// The server runs until ctx is canceled, then shuts down gracefully.
func StartServer(ctx context.Context, addr string) error {
	if addr == "" {
		return nil
	}
	var startErr error
	startOnce.Do(func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, "ok")
		})
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		errCh := make(chan error, 1)
		go func() {
			slog.Info("metrics server listening", "addr", addr, "path", "/metrics")
			err := srv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
				return
			}
			errCh <- nil
		}()
		go func() {
			<-ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		}()
		// Give the listener a brief moment to bind so we surface bind errors
		// (e.g., port in use) before the command starts hammering RPCs.
		select {
		case err := <-errCh:
			startErr = err
		case <-time.After(150 * time.Millisecond):
		}
	})
	return startErr
}
