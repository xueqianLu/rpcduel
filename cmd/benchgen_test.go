package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func resetBenchgenGlobals() {
	benchgenDataset = "dataset.json"
	benchgenRPCs = nil
	benchgenConcurrency = 10
	benchgenRequests = 1000
	benchgenDuration = 0
	benchgenTimeout = 30 * time.Second
	benchgenTraceTx = false
	benchgenTraceBlock = false
	benchgenOut = ""
	benchgenCSV = ""
	benchgenOutput = "text"
}

func TestRunBenchgen_ExportOnly(t *testing.T) {
	resetBenchgenGlobals()
	defer resetBenchgenGlobals()

	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "dataset.json")
	outPath := filepath.Join(dir, "bench.json")
	data := []byte(`{
  "meta": {"chain": "test", "rpc": "http://localhost", "generated_at": "2026-04-01T00:00:00Z"},
  "range": {"from": 1, "to": 2},
  "accounts": [
    {"address": "0xaaa", "tx_count": 1, "transactions": [{"hash": "0xtx1", "block_number": 1, "from": "0xaaa", "to": "0xbbb"}]},
    {"address": "0xbbb", "tx_count": 1, "transactions": [{"hash": "0xtx2", "block_number": 2, "from": "0xbbb", "to": "0xaaa"}]}
  ],
  "transactions": [
    {"hash": "0xtx1", "block_number": 1, "from": "0xaaa", "to": "0xbbb"},
    {"hash": "0xtx2", "block_number": 2, "from": "0xbbb", "to": "0xaaa"}
  ],
  "blocks": [
    {"number": 1, "tx_count": 1},
    {"number": 2, "tx_count": 1}
  ]
}`)
	if err := os.WriteFile(datasetPath, data, 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}

	benchgenDataset = datasetPath
	benchgenTraceTx = true
	benchgenOut = outPath

	if err := runBenchgen(nil, nil); err != nil {
		t.Fatalf("runBenchgen export-only: %v", err)
	}

	outData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read bench file: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(outData, &got); err != nil {
		t.Fatalf("parse bench file: %v", err)
	}
	if got["version"] != "1" {
		t.Fatalf("expected version 1, got %v", got["version"])
	}
	if _, ok := got["scenarios"]; !ok {
		t.Fatalf("expected scenarios in exported bench file, got %v", got)
	}
}
