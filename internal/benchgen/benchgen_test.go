// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package benchgen_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/benchgen"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

func sampleDataset() *dataset.Dataset {
	return &dataset.Dataset{
		Meta:  dataset.Meta{Chain: "test"},
		Range: dataset.Range{From: 100, To: 200},
		Accounts: []dataset.Account{
			{Address: "0xaaa", TxCount: 10, Transactions: []dataset.Transaction{{Hash: "0xtx1", BlockNumber: 150, From: "0xaaa", To: "0xbbb"}}},
			{Address: "0xbbb", TxCount: 5, Transactions: []dataset.Transaction{{Hash: "0xtx2", BlockNumber: 160, From: "0xbbb", To: "0xaaa"}}},
		},
		Transactions: []dataset.Transaction{
			{Hash: "0xtx1", BlockNumber: 150, From: "0xaaa", To: "0xbbb"},
			{Hash: "0xtx2", BlockNumber: 160, From: "0xbbb", To: "0xaaa"},
		},
		Blocks: []dataset.Block{
			{Number: 150, TxCount: 3},
			{Number: 160, TxCount: 2},
		},
	}
}

func TestGenerate_Scenarios(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true, TraceBlock: true})

	if len(bf.Scenarios) == 0 {
		t.Fatal("expected at least one scenario")
	}

	if bf.Version != "1" {
		t.Errorf("expected version '1', got %q", bf.Version)
	}

	// Check expected scenario names are present
	names := make(map[string]bool)
	for _, s := range bf.Scenarios {
		names[s.Name] = true
	}
	required := []string{"balance", "transaction_by_hash", "transaction_receipt",
		"block_by_number", "get_logs", "debug_trace_transaction", "debug_trace_block", "mixed_balance"}
	for _, req := range required {
		if !names[req] {
			t.Errorf("expected scenario %q not found", req)
		}
	}
}

func TestGenerate_Weights(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true, TraceBlock: true})

	totalWeight := 0.0
	for _, s := range bf.Scenarios {
		if s.Weight <= 0 {
			t.Errorf("scenario %q has non-positive weight %v", s.Name, s.Weight)
		}
		totalWeight += s.Weight
	}
	// Weights should sum to approximately 1.0
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("scenario weights sum to %v, expected ~1.0", totalWeight)
	}
}

func TestGenerate_Requests(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true, TraceBlock: true})

	for _, s := range bf.Scenarios {
		if len(s.Requests) == 0 {
			t.Errorf("scenario %q has no requests", s.Name)
		}
		for _, r := range s.Requests {
			if r.Method == "" {
				t.Errorf("scenario %q has request with empty method", s.Name)
			}
		}
	}
}

func TestFlattenRequests(t *testing.T) {
	bf := benchgen.Generate(sampleDataset(), nil)
	flat := bf.FlattenRequests()
	if len(flat) == 0 {
		t.Error("expected flattened requests to be non-empty")
	}
}

func TestSaveLoadBenchFile(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true, TraceBlock: true})
	path := t.TempDir() + "/bench.json"

	if err := benchgen.SaveBenchFile(path, bf); err != nil {
		t.Fatalf("SaveBenchFile: %v", err)
	}

	loaded, err := benchgen.LoadBenchFile(path)
	if err != nil {
		t.Fatalf("LoadBenchFile: %v", err)
	}

	if loaded.Version != bf.Version {
		t.Errorf("version mismatch: got %q want %q", loaded.Version, bf.Version)
	}
	if len(loaded.Scenarios) != len(bf.Scenarios) {
		t.Errorf("scenario count mismatch: got %d want %d", len(loaded.Scenarios), len(bf.Scenarios))
	}
}

func TestWeightedRequests(t *testing.T) {
	bf := benchgen.Generate(sampleDataset(), nil)
	n := 50
	reqs := bf.WeightedRequests(n, nil)
	if len(reqs) != n {
		t.Errorf("expected %d weighted requests, got %d", n, len(reqs))
	}
	for _, r := range reqs {
		if r.Method == "" {
			t.Error("weighted request has empty method")
		}
	}
}

func TestWeightedTaggedRequests(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true, TraceBlock: true})
	n := 100
	tagged := bf.WeightedTaggedRequests(n, nil)
	if len(tagged) != n {
		t.Errorf("expected %d tagged requests, got %d", n, len(tagged))
	}
	scenariosSeen := make(map[string]bool)
	for _, r := range tagged {
		if r.Method == "" {
			t.Error("tagged request has empty method")
		}
		if r.Scenario == "" {
			t.Error("tagged request has empty scenario name")
		}
		scenariosSeen[r.Scenario] = true
	}
	// With n=100 and multiple scenarios, we should see at least 2 distinct scenarios.
	if len(scenariosSeen) < 2 {
		t.Errorf("expected at least 2 distinct scenarios, got %d: %v", len(scenariosSeen), scenariosSeen)
	}
}

func TestGenerate_DefaultTraceDisabled(t *testing.T) {
	bf := benchgen.Generate(sampleDataset(), nil)
	names := make(map[string]bool)
	for _, s := range bf.Scenarios {
		names[s.Name] = true
	}
	if names["debug_trace_transaction"] || names["debug_trace_block"] {
		t.Fatalf("expected trace scenarios to be disabled by default, got %v", names)
	}
}

func TestGenerate_TraceOptions(t *testing.T) {
	both := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true, TraceBlock: true})
	names := make(map[string]bool)
	for _, s := range both.Scenarios {
		names[s.Name] = true
	}
	if !names["debug_trace_transaction"] || !names["debug_trace_block"] {
		t.Fatalf("expected both trace scenarios when enabled, got %v", names)
	}
}

func TestGenerate_MixedBalanceUsesHistoricalBlocks(t *testing.T) {
	bf := benchgen.Generate(sampleDataset(), rand.New(rand.NewSource(1)))
	var balanceReqs, mixedReqs []benchgen.Request
	for _, s := range bf.Scenarios {
		switch s.Name {
		case "balance":
			balanceReqs = s.Requests
		case "mixed_balance":
			mixedReqs = s.Requests
		}
	}
	if len(mixedReqs) == 0 {
		t.Fatal("expected mixed_balance requests")
	}
	balanceSet := make(map[string]bool)
	for _, r := range balanceReqs {
		balanceSet[fmt.Sprint(r.Params)] = true
		if got := r.Params[1]; got != "latest" {
			t.Fatalf("expected balance scenario to use latest, got %v", got)
		}
	}
	for _, r := range mixedReqs {
		if r.Params[1] == "latest" {
			t.Fatalf("expected mixed_balance to use historical block tags, got latest in %v", r.Params)
		}
		if balanceSet[fmt.Sprint(r.Params)] {
			t.Fatalf("expected mixed_balance request %v to differ from balance scenario", r.Params)
		}
	}
}

func TestWeightedTaggedSampler(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{TraceTransaction: true})
	s := bf.NewWeightedTaggedSampler(rand.New(rand.NewSource(99)))
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		req := s.Next()
		if req.Method == "" {
			t.Fatal("expected sampled method")
		}
		if req.Scenario == "" {
			t.Fatal("expected sampled scenario")
		}
		seen[req.Scenario] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected sampler to visit multiple scenarios, got %v", seen)
	}
}

func TestGenerate_OnlySelectedScenarios(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{
		Only: map[string]bool{
			"balance":                 true,
			"debug_trace_transaction": true,
		},
	})
	if len(bf.Scenarios) != 2 {
		t.Fatalf("expected exactly 2 scenarios, got %d", len(bf.Scenarios))
	}
	names := make(map[string]bool)
	for _, s := range bf.Scenarios {
		names[s.Name] = true
	}
	if !names["balance"] || !names["debug_trace_transaction"] {
		t.Fatalf("expected only selected scenarios, got %v", names)
	}
}

func TestGenerate_OnlyTraceDoesNotRequireTraceFlags(t *testing.T) {
	bf := benchgen.GenerateWithOptions(sampleDataset(), nil, benchgen.Options{
		Only: map[string]bool{"debug_trace_block": true},
	})
	if len(bf.Scenarios) != 1 || bf.Scenarios[0].Name != "debug_trace_block" {
		t.Fatalf("expected only debug_trace_block scenario, got %+v", bf.Scenarios)
	}
}

func TestGenerate_GetLogsUsesBlockRangeAndAddressFilter(t *testing.T) {
	bf := benchgen.Generate(sampleDataset(), nil)
	var logs []benchgen.Request
	for _, s := range bf.Scenarios {
		if s.Name == "get_logs" {
			logs = s.Requests
			break
		}
	}
	if len(logs) == 0 {
		t.Fatal("expected get_logs scenario")
	}
	for _, req := range logs {
		if req.Method != "eth_getLogs" {
			t.Fatalf("expected eth_getLogs, got %s", req.Method)
		}
		if len(req.Params) != 1 {
			t.Fatalf("expected single filter param, got %v", req.Params)
		}
		filter, ok := req.Params[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected filter object, got %#v", req.Params[0])
		}
		if filter["fromBlock"] != filter["toBlock"] {
			t.Fatalf("expected single-block get_logs range, got %v", filter)
		}
		if _, ok := filter["address"]; !ok {
			t.Fatalf("expected get_logs filter to include an address when tx.to is available, got %v", filter)
		}
	}
}
