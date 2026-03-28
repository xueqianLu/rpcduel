package dataset_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

func TestCollectorCollectStreamsValidDatasetAndDeduplicates(t *testing.T) {
	blockResults := map[string]any{
		"0x1": map[string]any{
			"number":     "0x1",
			"hash":       "0xblock1",
			"parentHash": "0xparent0",
			"timestamp":  "0x64",
			"miner":      "0xAaA",
			"transactions": []any{
				map[string]any{
					"hash":        "0xtx1",
					"blockNumber": "0x1",
					"from":        "0xBbB",
					"to":          "0xCcC",
				},
				map[string]any{
					"hash":        "0xtx1",
					"blockNumber": "0x1",
					"from":        "0xbbb",
					"to":          "0xccc",
				},
			},
		},
		"0x2": map[string]any{
			"number":     "0x2",
			"hash":       "0xblock2",
			"parentHash": "0xblock1",
			"timestamp":  "0x65",
			"miner":      "0xaaa",
			"transactions": []any{
				map[string]any{
					"hash":        "0xtx2",
					"blockNumber": "0x2",
					"from":        "0xbbb",
					"to":          "0xDdD",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request rpc.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		blockTag, _ := request.Params[0].(string)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result":  blockResults[blockTag],
		})
	}))
	defer server.Close()

	provider := rpc.NewProvider(
		rpc.Target{Name: "fixture", URL: server.URL},
		5*time.Second,
		rpc.WithRetryPolicy(rpc.RetryPolicy{
			MaxAttempts: 1,
			BaseBackoff: time.Millisecond,
			MaxBackoff:  time.Millisecond,
		}),
	)

	var output bytes.Buffer
	summary, err := dataset.NewCollector(provider).Collect(context.Background(), 1, 2, &output)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if summary.Blocks != 2 || summary.Transactions != 2 || summary.Addresses != 4 {
		t.Fatalf("Collect() summary = %+v, want blocks=2 txs=2 addrs=4", summary)
	}

	var file dataset.File
	if err := json.Unmarshal(output.Bytes(), &file); err != nil {
		t.Fatalf("Collect() produced invalid JSON: %v\n%s", err, output.String())
	}

	if file.Meta.Source != "fixture" || file.Meta.FromBlock != 1 || file.Meta.ToBlock != 2 {
		t.Fatalf("Collect() meta = %+v", file.Meta)
	}

	recordCounts := map[string]int{}
	var firstBlock *dataset.BlockRecord
	for _, record := range file.Records {
		recordCounts[record.Type]++
		if record.Type == dataset.RecordTypeBlock && record.Block != nil && record.Block.Number == 1 {
			firstBlock = record.Block
		}
	}

	if recordCounts[dataset.RecordTypeBlock] != 2 ||
		recordCounts[dataset.RecordTypeTransaction] != 2 ||
		recordCounts[dataset.RecordTypeAddress] != 4 {
		t.Fatalf("Collect() record counts = %+v", recordCounts)
	}

	if firstBlock == nil {
		t.Fatal("Collect() missing first block record")
	}

	if firstBlock.Hash != "0xblock1" || firstBlock.ParentHash != "0xparent0" || firstBlock.Timestamp != 100 || firstBlock.TxCount != 2 {
		t.Fatalf("Collect() first block = %+v", firstBlock)
	}
}
