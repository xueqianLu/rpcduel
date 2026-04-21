// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package benchgen

import (
	"math/rand"
	"sort"
	"strings"
)

// FilterMethods returns a copy of bf with only the requested RPC methods
// retained. Method names are matched case-insensitively. Scenarios that
// become empty after filtering are dropped. If methods is empty, bf is
// returned unchanged.
func FilterMethods(bf *BenchFile, methods []string) *BenchFile {
	if len(methods) == 0 {
		return bf
	}
	keep := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		m = strings.TrimSpace(strings.ToLower(m))
		if m != "" {
			keep[m] = struct{}{}
		}
	}
	if len(keep) == 0 {
		return bf
	}
	out := &BenchFile{Version: bf.Version}
	for _, s := range bf.Scenarios {
		var reqs []Request
		for _, r := range s.Requests {
			if _, ok := keep[strings.ToLower(r.Method)]; ok {
				reqs = append(reqs, r)
			}
		}
		if len(reqs) == 0 {
			continue
		}
		out.Scenarios = append(out.Scenarios, Scenario{
			Name:     s.Name,
			Weight:   s.Weight,
			Requests: reqs,
		})
	}
	return out
}

// Sample downsamples each scenario to the given fraction (0 < frac <= 1) using
// rng for reproducibility. Each request is kept independently with probability
// frac; if a scenario would end up empty, one randomly chosen request is kept
// to preserve coverage. If frac >= 1 (or <= 0) bf is returned unchanged.
func Sample(bf *BenchFile, frac float64, rng *rand.Rand) *BenchFile {
	if frac >= 1 || frac <= 0 {
		return bf
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}
	out := &BenchFile{Version: bf.Version}
	for _, s := range bf.Scenarios {
		if len(s.Requests) == 0 {
			continue
		}
		// Stable order for deterministic sampling regardless of map iteration.
		reqs := make([]Request, len(s.Requests))
		copy(reqs, s.Requests)
		sort.SliceStable(reqs, func(i, j int) bool {
			if reqs[i].Method != reqs[j].Method {
				return reqs[i].Method < reqs[j].Method
			}
			return false
		})
		var kept []Request
		for _, r := range reqs {
			if rng.Float64() < frac {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 {
			kept = []Request{reqs[rng.Intn(len(reqs))]}
		}
		out.Scenarios = append(out.Scenarios, Scenario{
			Name:     s.Name,
			Weight:   s.Weight,
			Requests: kept,
		})
	}
	return out
}
