// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package thresholds

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/config"
	"github.com/xueqianLu/rpcduel/internal/replay"
)

func TestEvalBench(t *testing.T) {
	sums := []bench.Summary{{
		Endpoint: "a", Total: 100, Errors: 10, ErrorRate: 0.1, QPS: 50,
		P95: 250 * time.Millisecond, P99: 600 * time.Millisecond,
	}}
	br := EvalBench(sums, config.BenchThresholds{P95Ms: 200, P99Ms: 500, ErrorRate: 0.05, MinQPS: 100})
	if len(br) != 4 {
		t.Fatalf("expected 4 breaches, got %d: %v", len(br), br)
	}
	pass := EvalBench(sums, config.BenchThresholds{P95Ms: 1000, P99Ms: 1000, ErrorRate: 0.5, MinQPS: 1})
	if len(pass) != 0 {
		t.Errorf("expected pass, got %v", pass)
	}
}

func TestEvalDuel(t *testing.T) {
	sums := []bench.Summary{{Endpoint: "a", P99: 600 * time.Millisecond, ErrorRate: 0.02}}
	br := EvalDuel(0.05, sums, config.DuelThresholds{DiffRate: 0.01, P99Ms: 500, ErrorRate: 0.01})
	if len(br) != 3 {
		t.Errorf("expected 3 breaches, got %d: %v", len(br), br)
	}
}

func TestEvalDiff(t *testing.T) {
	br := EvalDiff(100, 5, config.DiffThresholds{DiffRate: 0.01, MaxDiffs: 3})
	if len(br) != 2 {
		t.Errorf("expected 2 breaches, got %d", len(br))
	}
}

func TestEvalReplay(t *testing.T) {
	r := &replay.Result{TotalRequests: 100, SuccessRequests: 90, Diffs: make([]replay.FoundDiff, 4)}
	br := EvalReplay(r, config.ReplayThresholds{MismatchRate: 0.01, ErrorRate: 0.05, MaxMismatch: 2})
	if len(br) != 3 {
		t.Errorf("expected 3 breaches, got %d: %v", len(br), br)
	}
}

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	if Print(&buf, nil, false) {
		t.Error("unconfigured should not fail")
	}
	if buf.Len() != 0 {
		t.Errorf("unconfigured wrote output: %q", buf.String())
	}
	buf.Reset()
	if Print(&buf, nil, true) {
		t.Error("zero breaches should not fail")
	}
	if !strings.Contains(buf.String(), "PASS") {
		t.Errorf("expected PASS, got %q", buf.String())
	}
	buf.Reset()
	failed := Print(&buf, []Breach{{Metric: "x", Limit: 1, Actual: 2}}, true)
	if !failed {
		t.Error("breach should fail")
	}
	if !strings.Contains(buf.String(), "FAIL") {
		t.Errorf("expected FAIL, got %q", buf.String())
	}
}
