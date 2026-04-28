// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package tracerflag centralises how the rpcduel CLI exposes the
// `tracer` parameter for the debug_trace* family of RPC methods.
//
// Geth (and most clients that copy its tracer registry — erigon, reth,
// op-geth, bsc, etc.) accepts an optional second argument shaped like
//
//	{ "tracer": "<name>", "tracerConfig": { ... } }
//
// When omitted the node falls back to the default `structLogger`, which
// is extremely verbose — typically not what users actually want for
// benchmarking or cross-node diffing. rpcduel therefore defaults to the
// industry-standard `callTracer`. Pass `--tracer default` to restore the
// node's built-in default behaviour (empty config object).
package tracerflag

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Default is the tracer name rpcduel picks when the user does not pass
// --tracer. callTracer is the de-facto standard tracer used by
// indexers, explorers, MEV and audit tooling.
const Default = "callTracer"

// Build returns the second argument that should be passed to
// debug_traceTransaction / debug_traceBlockByNumber given the user's
// raw --tracer and --tracer-config values.
//
//   - tracer == "" → use Default ("callTracer").
//   - tracer == "default" / "none" → return an empty map, i.e. let the
//     node use its built-in default tracer (preserves pre-flag behaviour).
//   - tracerConfig, when non-empty, must be a JSON object; it is placed
//     under the "tracerConfig" key of the result.
func Build(tracer, tracerConfig string) (map[string]interface{}, error) {
	tracer = strings.TrimSpace(tracer)
	if tracer == "" {
		tracer = Default
	}
	if tracer == "default" || tracer == "none" {
		if strings.TrimSpace(tracerConfig) != "" {
			return nil, fmt.Errorf("--tracer-config cannot be used with --tracer=%s", tracer)
		}
		return map[string]interface{}{}, nil
	}
	out := map[string]interface{}{"tracer": tracer}
	if tc := strings.TrimSpace(tracerConfig); tc != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(tc), &parsed); err != nil {
			return nil, fmt.Errorf("--tracer-config: invalid JSON object: %w", err)
		}
		out["tracerConfig"] = parsed
	}
	return out, nil
}

// FlagUsage returns help text suitable for --tracer.
func FlagUsage() string {
	return `tracer name to pass to debug_trace* RPCs. Common values:` +
		` "callTracer" (default — full call tree),` +
		` "prestateTracer" (state read by the tx; with tracer-config {"diffMode":true} → state diff),` +
		` "4byteTracer" (selector frequency),` +
		` "noopTracer", "muxTracer", "flatCallTracer".` +
		` Use "default" to keep the node's built-in tracer (verbose structLogger).`
}

// ConfigFlagUsage returns help text suitable for --tracer-config.
func ConfigFlagUsage() string {
	return `JSON object passed as the "tracerConfig" field, e.g. ` +
		`'{"onlyTopCall":true}' for callTracer or '{"diffMode":true}' ` +
		`for prestateTracer.`
}
