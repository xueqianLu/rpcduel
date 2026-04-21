// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBenchStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	m := NewMetrics("http://x")
	m.Record(2*time.Millisecond, false)
	m.Record(5*time.Millisecond, false)
	m.Record(10*time.Millisecond, true)

	st := &State{
		Mode:        "requests-single-method",
		Method:      "eth_blockNumber",
		ParamsJSON:  "[]",
		Endpoints:   []string{"http://x"},
		TargetTotal: 100,
		Per:         []EndpointState{m.Snapshot()},
	}
	if err := SaveState(path, st); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Method != "eth_blockNumber" || loaded.TargetTotal != 100 {
		t.Errorf("mismatch: %+v", loaded)
	}
	if len(loaded.Per) != 1 || loaded.Per[0].Total != 3 || loaded.Per[0].Errors != 1 {
		t.Errorf("per-endpoint mismatch: %+v", loaded.Per)
	}

	m2 := NewMetrics("http://x")
	m2.RestoreFromSnapshot(loaded.Per[0])
	if m2.Total != 3 || m2.Errors != 1 {
		t.Errorf("restored counters wrong: total=%d errors=%d", m2.Total, m2.Errors)
	}
	if m2.Histogram() == nil || m2.Histogram().TotalCount() != 3 {
		t.Errorf("restored hist count = %d, want 3", m2.Histogram().TotalCount())
	}
	// Sanity: median should be in the recorded range.
	p50 := m2.Percentile(50)
	if p50 < 1*time.Millisecond || p50 > 20*time.Millisecond {
		t.Errorf("p50 = %v out of expected range", p50)
	}
}

func TestBenchLoadStateMissing(t *testing.T) {
	_, err := LoadState(filepath.Join(t.TempDir(), "nope.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}
