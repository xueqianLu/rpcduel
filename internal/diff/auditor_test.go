package diff_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

func TestAuditorUsesConcurrentBlockWorkers(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32

	newServer := func(trackConcurrency bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request rpc.Request
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if trackConcurrency && request.Method == "eth_getBlockByNumber" {
				current := inFlight.Add(1)
				for {
					maximum := maxInFlight.Load()
					if current <= maximum || maxInFlight.CompareAndSwap(maximum, current) {
						break
					}
				}

				select {
				case started <- struct{}{}:
				default:
				}
				<-release
				inFlight.Add(-1)
			}

			blockTag := "0x0"
			if len(request.Params) > 0 {
				blockTag, _ = request.Params[0].(string)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result": map[string]any{
					"number":       blockTag,
					"hash":         blockTag + "-hash",
					"parentHash":   "0xparent",
					"timestamp":    "0x1",
					"transactions": []any{},
				},
			})
		}))
	}

	baselineServer := newServer(true)
	defer baselineServer.Close()
	peerServer := newServer(false)
	defer peerServer.Close()

	baseline := diff.Endpoint{
		Target: rpc.Target{Name: "baseline", URL: baselineServer.URL},
		Provider: rpc.NewProvider(
			rpc.Target{Name: "baseline", URL: baselineServer.URL},
			5*time.Second,
			rpc.WithRetryPolicy(rpc.RetryPolicy{
				MaxAttempts: 1,
				BaseBackoff: time.Millisecond,
				MaxBackoff:  time.Millisecond,
			}),
		),
	}
	peer := diff.Endpoint{
		Target: rpc.Target{Name: "peer", URL: peerServer.URL},
		Provider: rpc.NewProvider(
			rpc.Target{Name: "peer", URL: peerServer.URL},
			5*time.Second,
			rpc.WithRetryPolicy(rpc.RetryPolicy{
				MaxAttempts: 1,
				BaseBackoff: time.Millisecond,
				MaxBackoff:  time.Millisecond,
			}),
		),
	}

	done := make(chan struct {
		report *diff.Report
		err    error
	}, 1)
	go func() {
		report, err := diff.NewAuditor(baseline, []diff.Endpoint{peer}, diff.DefaultOptions(), diff.WithConcurrency(2)).AuditBlockRange(context.Background(), 1, 2)
		done <- struct {
			report *diff.Report
			err    error
		}{report: report, err: err}
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent diff requests")
		}
	}

	close(release)

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("AuditBlockRange() error = %v", result.err)
		}
		if result.report == nil {
			t.Fatal("AuditBlockRange() report = nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for diff audit to finish")
	}

	if maxInFlight.Load() < 2 {
		t.Fatalf("max concurrent block audits = %d, want at least 2", maxInFlight.Load())
	}
}
