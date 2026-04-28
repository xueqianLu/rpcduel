// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package benchgen generates benchmark scenario files from a dataset.
// The output JSON can be fed directly into `rpcduel bench --input`.
package benchgen

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/xueqianLu/rpcduel/internal/dataset"
)

// Options controls which scenarios are generated.
type Options struct {
	TraceTransaction bool
	TraceBlock       bool
	Only             map[string]bool
	// TracerConfig is the second argument passed to debug_traceTransaction
	// and debug_traceBlockByNumber. When nil, an empty map (== node default
	// tracer) is sent. Build it with internal/tracerflag.
	TracerConfig map[string]interface{}
}

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

type scenarioEntry struct {
	s          *Scenario
	cumulative float64
}

// WeightedSampler samples requests from scenarios according to their weights.
type WeightedSampler struct {
	entries []scenarioEntry
	flat    []Request
	total   float64
	rng     *rand.Rand
}

// WeightedTaggedSampler is like WeightedSampler but preserves the scenario name.
type WeightedTaggedSampler struct {
	entries []scenarioEntry
	flat    []Request
	total   float64
	rng     *rand.Rand
}

// FlattenRequests returns all requests from all scenarios as a flat slice.
func (bf *BenchFile) FlattenRequests() []Request {
	var out []Request
	for _, s := range bf.Scenarios {
		out = append(out, s.Requests...)
	}
	return out
}

// NewWeightedSampler creates a request sampler distributed by scenario weight.
func (bf *BenchFile) NewWeightedSampler(rng *rand.Rand) *WeightedSampler {
	entries, total := buildScenarioEntries(bf)
	return &WeightedSampler{
		entries: entries,
		flat:    bf.FlattenRequests(),
		total:   total,
		rng:     defaultRNG(rng),
	}
}

// NewWeightedTaggedSampler creates a tagged request sampler distributed by scenario weight.
func (bf *BenchFile) NewWeightedTaggedSampler(rng *rand.Rand) *WeightedTaggedSampler {
	entries, total := buildScenarioEntries(bf)
	return &WeightedTaggedSampler{
		entries: entries,
		flat:    bf.FlattenRequests(),
		total:   total,
		rng:     defaultRNG(rng),
	}
}

// WeightedTaggedRequests is like WeightedRequests but each returned element
// also carries the name of the scenario it was drawn from.
func (bf *BenchFile) WeightedTaggedRequests(n int, rng *rand.Rand) []TaggedRequest {
	sampler := bf.NewWeightedTaggedSampler(rng)
	out := make([]TaggedRequest, n)
	for i := range out {
		out[i] = sampler.Next()
	}
	return out
}

// WeightedRequests builds a slice of n requests distributed across scenarios
// according to each scenario's Weight. Scenarios with zero weight are skipped.
// If the total weight is zero, it falls back to FlattenRequests.
func (bf *BenchFile) WeightedRequests(n int, rng *rand.Rand) []Request {
	sampler := bf.NewWeightedSampler(rng)
	out := make([]Request, n)
	for i := range out {
		out[i] = sampler.Next()
	}
	return out
}

// Next returns the next weighted request.
func (s *WeightedSampler) Next() Request {
	if s.total == 0 || len(s.entries) == 0 {
		if len(s.flat) == 0 {
			return Request{}
		}
		return s.flat[s.rng.Intn(len(s.flat))]
	}
	r := s.rng.Float64() * s.total
	for _, e := range s.entries {
		if r <= e.cumulative {
			reqs := e.s.Requests
			return reqs[s.rng.Intn(len(reqs))]
		}
	}
	last := s.entries[len(s.entries)-1].s.Requests
	return last[s.rng.Intn(len(last))]
}

// Next returns the next weighted tagged request.
func (s *WeightedTaggedSampler) Next() TaggedRequest {
	if s.total == 0 || len(s.entries) == 0 {
		if len(s.flat) == 0 {
			return TaggedRequest{}
		}
		return TaggedRequest{Request: s.flat[s.rng.Intn(len(s.flat))]}
	}
	r := s.rng.Float64() * s.total
	for _, e := range s.entries {
		if r <= e.cumulative {
			reqs := e.s.Requests
			return TaggedRequest{Request: reqs[s.rng.Intn(len(reqs))], Scenario: e.s.Name}
		}
	}
	last := s.entries[len(s.entries)-1].s
	return TaggedRequest{Request: last.Requests[s.rng.Intn(len(last.Requests))], Scenario: last.Name}
}

// Generate creates a BenchFile from the given dataset.
// rng is used for mixing cold/hot account selection; pass nil for a default source.
func Generate(ds *dataset.Dataset, rng *rand.Rand) *BenchFile {
	return GenerateWithOptions(ds, rng, Options{})
}

// GenerateWithOptions creates a BenchFile from the given dataset.
func GenerateWithOptions(ds *dataset.Dataset, rng *rand.Rand, opts Options) *BenchFile {
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
		if opts.enabled("balance") {
			for _, a := range ds.Accounts {
				s.Requests = append(s.Requests, Request{
					Method: "eth_getBalance",
					Params: []interface{}{a.Address, "latest"},
				})
			}
		}
		if opts.enabled("balance") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getTransactionCount
	{
		s := Scenario{Name: "transaction_count", Weight: 0.10}
		if opts.enabled("transaction_count") {
			for _, a := range ds.Accounts {
				s.Requests = append(s.Requests, Request{
					Method: "eth_getTransactionCount",
					Params: []interface{}{a.Address, "latest"},
				})
			}
		}
		if opts.enabled("transaction_count") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getTransactionByHash
	{
		s := Scenario{Name: "transaction_by_hash", Weight: 0.15}
		if opts.enabled("transaction_by_hash") {
			for _, tx := range ds.Transactions {
				s.Requests = append(s.Requests, Request{
					Method: "eth_getTransactionByHash",
					Params: []interface{}{tx.Hash},
				})
			}
		}
		if opts.enabled("transaction_by_hash") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getTransactionReceipt
	{
		s := Scenario{Name: "transaction_receipt", Weight: 0.15}
		if opts.enabled("transaction_receipt") {
			for _, tx := range ds.Transactions {
				s.Requests = append(s.Requests, Request{
					Method: "eth_getTransactionReceipt",
					Params: []interface{}{tx.Hash},
				})
			}
		}
		if opts.enabled("transaction_receipt") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// eth_getBlockByNumber
	{
		s := Scenario{Name: "block_by_number", Weight: 0.10}
		if opts.enabled("block_by_number") {
			for _, b := range ds.Blocks {
				s.Requests = append(s.Requests, Request{
					Method: "eth_getBlockByNumber",
					Params: []interface{}{fmt.Sprintf("0x%x", b.Number), false},
				})
			}
		}
		if opts.enabled("block_by_number") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	// -----------------------------------------------------------------------
	// Complex scenarios (total weight: 0.30)
	// -----------------------------------------------------------------------

	// eth_getLogs – query each block's range individually
	{
		s := Scenario{Name: "get_logs", Weight: 0.10}
		if opts.enabled("get_logs") {
			logAddresses := logAddressesByBlock(ds)
			for _, b := range ds.Blocks {
				hex := fmt.Sprintf("0x%x", b.Number)
				filter := map[string]interface{}{
					"fromBlock": hex,
					"toBlock":   hex,
				}
				if addr := logAddresses[b.Number]; addr != "" {
					filter["address"] = addr
				}
				s.Requests = append(s.Requests, Request{
					Method: "eth_getLogs",
					Params: []interface{}{filter},
				})
			}
		}
		if opts.enabled("get_logs") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	if opts.enabled("debug_trace_transaction") {
		// debug_traceTransaction
		{
			s := Scenario{Name: "debug_trace_transaction", Weight: 0.10}
			for _, tx := range ds.Transactions {
				s.Requests = append(s.Requests, Request{
					Method: "debug_traceTransaction",
					Params: []interface{}{tx.Hash, opts.tracerCfg()},
				})
			}
			if len(s.Requests) > 0 {
				bf.Scenarios = append(bf.Scenarios, s)
			}
		}
	}

	if opts.enabled("debug_trace_block") {
		// debug_traceBlockByNumber
		{
			s := Scenario{Name: "debug_trace_block", Weight: 0.05}
			for _, b := range ds.Blocks {
				s.Requests = append(s.Requests, Request{
					Method: "debug_traceBlockByNumber",
					Params: []interface{}{fmt.Sprintf("0x%x", b.Number), opts.tracerCfg()},
				})
			}
			if len(s.Requests) > 0 {
				bf.Scenarios = append(bf.Scenarios, s)
			}
		}
	}

	// Mixed scenario: shuffled accounts queried at historical block heights,
	// which avoids duplicating the plain latest-balance scenario.
	{
		s := Scenario{Name: "mixed_balance", Weight: 0.05}
		if opts.enabled("mixed_balance") {
			accounts := make([]dataset.Account, len(ds.Accounts))
			copy(accounts, ds.Accounts)
			rng.Shuffle(len(accounts), func(i, j int) { accounts[i], accounts[j] = accounts[j], accounts[i] })
			historical := historicalAccountBlocks(ds)
			for _, a := range accounts {
				blocks := historical[strings.ToLower(a.Address)]
				if len(blocks) == 0 {
					continue
				}
				blockParam := fmt.Sprintf("0x%x", blocks[rng.Intn(len(blocks))])
				s.Requests = append(s.Requests, Request{
					Method: "eth_getBalance",
					Params: []interface{}{a.Address, blockParam},
				})
			}
		}
		if opts.enabled("mixed_balance") && len(s.Requests) > 0 {
			bf.Scenarios = append(bf.Scenarios, s)
		}
	}

	return bf
}

func defaultRNG(rng *rand.Rand) *rand.Rand {
	if rng == nil {
		return rand.New(rand.NewSource(42))
	}
	return rng
}

func buildScenarioEntries(bf *BenchFile) ([]scenarioEntry, float64) {
	var entries []scenarioEntry
	total := 0.0
	for i := range bf.Scenarios {
		s := &bf.Scenarios[i]
		if len(s.Requests) == 0 || s.Weight <= 0 {
			continue
		}
		total += s.Weight
		entries = append(entries, scenarioEntry{s: s, cumulative: total})
	}
	return entries, total
}

func historicalAccountBlocks(ds *dataset.Dataset) map[string][]int64 {
	seen := make(map[string]map[int64]bool)
	out := make(map[string][]int64)
	add := func(addr string, block int64) {
		addr = strings.ToLower(addr)
		if addr == "" || block <= 0 {
			return
		}
		if seen[addr] == nil {
			seen[addr] = make(map[int64]bool)
		}
		if seen[addr][block] {
			return
		}
		seen[addr][block] = true
		out[addr] = append(out[addr], block)
	}

	for _, a := range ds.Accounts {
		for _, tx := range a.Transactions {
			add(a.Address, tx.BlockNumber)
		}
	}
	for _, tx := range ds.Transactions {
		add(tx.From, tx.BlockNumber)
		add(tx.To, tx.BlockNumber)
	}
	return out
}

func logAddressesByBlock(ds *dataset.Dataset) map[int64]string {
	seen := make(map[int64]map[string]bool)
	out := make(map[int64]string)
	for _, tx := range ds.Transactions {
		addr := strings.TrimSpace(tx.To)
		if addr == "" {
			continue
		}
		lower := strings.ToLower(addr)
		if seen[tx.BlockNumber] == nil {
			seen[tx.BlockNumber] = make(map[string]bool)
		}
		if seen[tx.BlockNumber][lower] {
			continue
		}
		seen[tx.BlockNumber][lower] = true
		if out[tx.BlockNumber] == "" {
			out[tx.BlockNumber] = addr
		}
	}
	return out
}

func (o Options) enabled(target string) bool {
	if len(o.Only) > 0 {
		return o.Only[target]
	}
	switch target {
	case "debug_trace_transaction":
		return o.TraceTransaction
	case "debug_trace_block":
		return o.TraceBlock
	default:
		return true
	}
}

// tracerCfg returns the second argument for debug_trace* methods. When
// no tracer was configured we send an empty object so the node uses its
// built-in default tracer (preserves pre-flag behaviour).
func (o Options) tracerCfg() map[string]interface{} {
	if o.TracerConfig == nil {
		return map[string]interface{}{}
	}
	return o.TracerConfig
}
