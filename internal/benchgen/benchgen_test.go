package benchgen_test

import (
	"testing"

	"github.com/xueqianLu/rpcduel/internal/benchgen"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

func sampleDataset() *dataset.Dataset {
	return &dataset.Dataset{
		Meta: dataset.Meta{Chain: "test"},
		Range: dataset.Range{From: 100, To: 200},
		Accounts: []dataset.Account{
			{Address: "0xaaa", TxCount: 10},
			{Address: "0xbbb", TxCount: 5},
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
	bf := benchgen.Generate(sampleDataset(), nil)

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
		"block_by_number", "get_logs", "debug_trace_transaction"}
	for _, req := range required {
		if !names[req] {
			t.Errorf("expected scenario %q not found", req)
		}
	}
}

func TestGenerate_Weights(t *testing.T) {
	bf := benchgen.Generate(sampleDataset(), nil)

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
	bf := benchgen.Generate(sampleDataset(), nil)

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
	bf := benchgen.Generate(sampleDataset(), nil)
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
	bf := benchgen.Generate(sampleDataset(), nil)
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
