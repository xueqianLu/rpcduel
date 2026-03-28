package bench_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

func TestBuildRequests(t *testing.T) {
	file := &dataset.File{
		Records: []dataset.Record{
			{
				Type: dataset.RecordTypeBlock,
				Block: &dataset.BlockRecord{
					Number: 12,
				},
			},
			{
				Type: dataset.RecordTypeTransaction,
				Transaction: &dataset.TransactionData{
					Hash: "0xtx",
				},
			},
			{
				Type: dataset.RecordTypeAddress,
				Address: &dataset.AddressData{
					Address:        "0xabc",
					FirstSeenBlock: 12,
				},
			},
		},
	}

	requests := bench.BuildRequests(file)
	if len(requests) != 5 {
		t.Fatalf("BuildRequests() len = %d, want 5", len(requests))
	}

	if requests[0].Method != "eth_getBlockByNumber" {
		t.Fatalf("BuildRequests() first method = %q", requests[0].Method)
	}
	if requests[1].Method != "eth_getTransactionByHash" || requests[2].Method != "eth_getTransactionReceipt" {
		t.Fatalf("BuildRequests() transaction methods = %#v %#v", requests[1], requests[2])
	}
	if requests[3].Method != "eth_getBalance" || requests[4].Method != "eth_getTransactionCount" {
		t.Fatalf("BuildRequests() address methods = %#v %#v", requests[3], requests[4])
	}
}

func TestRunTracksHTTPFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request rpc.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if request.Method == "eth_getBalance" {
			http.Error(w, "temporary upstream issue", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  "0x1",
		})
	}))
	defer server.Close()

	provider := rpc.NewProvider(
		rpc.Target{Name: "local", URL: server.URL},
		5*time.Second,
		rpc.WithRetryPolicy(rpc.RetryPolicy{
			MaxAttempts: 1,
			BaseBackoff: time.Millisecond,
			MaxBackoff:  time.Millisecond,
		}),
	)

	summary, err := bench.Run(context.Background(), provider, []bench.Request{
		{Method: "eth_getBlockByNumber", Params: []any{"0x1", false}},
		{Method: "eth_getBalance", Params: []any{"0xabc", "0x1"}},
	}, 2, 2)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if summary.Requests != 2 || summary.Successes != 1 || summary.Failures != 1 {
		t.Fatalf("Run() summary = %+v", summary)
	}

	if summary.ErrorDistribution["http_503"] != 1 {
		t.Fatalf("Run() errors = %+v, want http_503=1", summary.ErrorDistribution)
	}
}
