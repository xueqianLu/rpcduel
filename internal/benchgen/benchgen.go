// Package benchgen generates benchmark scenario files from a dataset.
// The output JSON can be fed directly into `rpcduel bench --input`.
package benchgen

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"github.com/xueqianLu/rpcduel/internal/dataset"
)

// Request is a single JSON-RPC request in a scenario.
type Request struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// Scenario groups related bench requests under a name with a dispatch weight.
type Scenario struct {
	Name     string    `json:"name"`
	Weight   float64   `json:"weight"`
	Requests []Request `json:"requests"`
}

// BenchFile is the top-level structure written by benchgen.
type BenchFile struct {
	Version   string     `json:"version"`
	Scenarios []Scenario `json:"scenarios"`
}

// SaveBenchFile writes bf to the JSON file at path.
func SaveBenchFile(path string, bf *BenchFile) error {
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bench file: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write bench file %s: %w", path, err)
	}
	return nil
}

// LoadBenchFile reads a bench file from the JSON file at path.
func LoadBenchFile(path string) (*BenchFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bench file %s: %w", path, err)
	}
	var bf BenchFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parse bench file %s: %w", path, err)
	}
	return &bf, nil
}

// TaggedRequest is a Request paired with the name of the scenario it came from.
type TaggedRequest struct {
	Request
	Scenario string
}

// FlattenRequests returns all requests from all scenarios as a flat slice.
func (bf *BenchFile) FlattenRequests() []Request {
	var out []Request
	for _, s := range bf.Scenarios {
		out = append(out, s.Requests...)
	}
	return out
}

// WeightedTaggedRequests is like WeightedRequests but each returned element
// also carries the name of the scenario it was drawn from.
func (bf *BenchFile) WeightedTaggedRequests(n int, rng *rand.Rand) []TaggedRequest {
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	type scenarioEntry struct {
		s          *Scenario
		cumulative float64
	}
	var entries []scenarioEntry
	total := 0.0
	for i := range bf.Scenarios {
		s := &bf.Scenarios[i]
		if len(s.Requests) == 0 {
			continue
		}
		w := s.Weight
		if w <= 0 {
			continue
		}
		total += w
		entries = append(entries, scenarioEntry{s: s, cumulative: total})
	}

	if total == 0 || len(entries) == 0 {
		flat := bf.FlattenRequests()
		if len(flat) == 0 {
			return nil
		}
		out := make([]TaggedRequest, n)
		for i := range out {
			out[i] = TaggedRequest{Request: flat[i%len(flat)]}
		}
		return out
	}

	out := make([]TaggedRequest, n)
	for i := range out {
		r := rng.Float64() * total
		for _, e := range entries {
			if r <= e.cumulative {
				reqs := e.s.Requests
				out[i] = TaggedRequest{
					Request:  reqs[rng.Intn(len(reqs))],
					Scenario: e.s.Name,
				}
				break
			}
		}
	}
	return out
}

// WeightedRequests builds a slice of n requests distributed across scenarios
// according to each scenario's Weight. Scenarios with zero weight are skipped.
// If the total weight is zero, it falls back to FlattenRequests.
func (bf *BenchFile) WeightedRequests(n int, rng *rand.Rand) []Request {
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	// Compute total weight of scenarios that have requests.
	type scenarioEntry struct {
		s          *Scenario
		cumulative float64
	}
	var entries []scenarioEntry
	total := 0.0
	for i := range bf.Scenarios {
		s := &bf.Scenarios[i]
		if len(s.Requests) == 0 {
			continue
		}
		w := s.Weight
		if w <= 0 {
			continue
		}
		total += w
		entries = append(entries, scenarioEntry{s: s, cumulative: total})
	}

	if total == 0 || len(entries) == 0 {
		flat := bf.FlattenRequests()
		if len(flat) == 0 {
			return nil
		}
		out := make([]Request, n)
		for i := range out {
			out[i] = flat[i%len(flat)]
		}
		return out
	}

	out := make([]Request, n)
	for i := range out {
		r := rng.Float64() * total
		for _, e := range entries {
			if r <= e.cumulative {
				reqs := e.s.Requests
				out[i] = reqs[rng.Intn(len(reqs))]
				break
			}
		}
	}
	return out
}

// Generate creates a BenchFile from the given dataset.
// rng is used for mixing cold/hot account selection; pass nil for a default source.
func Generate(ds *dataset.Dataset, rng *rand.Rand) *BenchFile {
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	bf := &BenchFile{Version: "1"}

	// -----------------------------------------------------------------------
	// Basic scenarios (total weight: 0.70)
	// -----------------------------------------------------------------------

	// eth_getBalance
	{
		s := Scenario{Name: "balance", Weight: 0.20}
		for _, a := range ds.Accounts {
			s.Requests = append(s.Requests, Request{
				Method: "eth_getBalance",
				Params: []interface{}{a.Address, "latest"},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getTransactionCount
	{
		s := Scenario{Name: "transaction_count", Weight: 0.10}
		for _, a := range ds.Accounts {
			s.Requests = append(s.Requests, Request{
				Method: "eth_getTransactionCount",
				Params: []interface{}{a.Address, "latest"},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getTransactionByHash
	{
		s := Scenario{Name: "transaction_by_hash", Weight: 0.15}
		for _, tx := range ds.Transactions {
			s.Requests = append(s.Requests, Request{
				Method: "eth_getTransactionByHash",
				Params: []interface{}{tx.Hash},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getTransactionReceipt
	{
		s := Scenario{Name: "transaction_receipt", Weight: 0.15}
		for _, tx := range ds.Transactions {
			s.Requests = append(s.Requests, Request{
				Method: "eth_getTransactionReceipt",
				Params: []interface{}{tx.Hash},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getBlockByNumber
	{
		s := Scenario{Name: "block_by_number", Weight: 0.10}
		for _, b := range ds.Blocks {
			s.Requests = append(s.Requests, Request{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{fmt.Sprintf("0x%x", b.Number), false},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// -----------------------------------------------------------------------
	// Complex scenarios (total weight: 0.30)
	// -----------------------------------------------------------------------

	// eth_getLogs – query each block's range individually
	{
		s := Scenario{Name: "get_logs", Weight: 0.10}
		for _, b := range ds.Blocks {
			hex := fmt.Sprintf("0x%x", b.Number)
			s.Requests = append(s.Requests, Request{
				Method: "eth_getLogs",
				Params: []interface{}{map[string]interface{}{
					"fromBlock": hex,
					"toBlock":   hex,
				}},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// debug_traceTransaction
	{
		s := Scenario{Name: "debug_trace_transaction", Weight: 0.10}
		for _, tx := range ds.Transactions {
			s.Requests = append(s.Requests, Request{
				Method: "debug_traceTransaction",
				Params: []interface{}{tx.Hash, map[string]interface{}{}},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// debug_traceBlockByNumber
	{
		s := Scenario{Name: "debug_trace_block", Weight: 0.05}
		for _, b := range ds.Blocks {
			s.Requests = append(s.Requests, Request{
				Method: "debug_traceBlockByNumber",
				Params: []interface{}{fmt.Sprintf("0x%x", b.Number), map[string]interface{}{}},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// Mixed scenario: shuffle accounts (hot + cold mix) for eth_getBalance
	{
		s := Scenario{Name: "mixed_balance", Weight: 0.05}
		accounts := make([]dataset.Account, len(ds.Accounts))
		copy(accounts, ds.Accounts)
		rng.Shuffle(len(accounts), func(i, j int) { accounts[i], accounts[j] = accounts[j], accounts[i] })
		for _, a := range accounts {
			blockParam := "latest"
			s.Requests = append(s.Requests, Request{
				Method: "eth_getBalance",
				Params: []interface{}{a.Address, blockParam},
			})
		}
		if len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	return bf
}
