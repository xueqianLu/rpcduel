// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s := &State{
		EndpointA:       "http://a",
		EndpointB:       "http://b",
		DoneKeys:        []string{"k2", "k1"},
		TotalRequests:   10,
		SuccessRequests: 8,
		Unsupported:     1,
		Diffs:           []FoundDiff{{Method: "eth_getBalance"}},
	}
	if err := SaveState(path, s); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.SchemaVersion != stateSchemaVersion {
		t.Errorf("schema version = %d, want %d", got.SchemaVersion, stateSchemaVersion)
	}
	if got.TotalRequests != 10 || got.SuccessRequests != 8 || got.Unsupported != 1 {
		t.Errorf("counters mismatch: %+v", got)
	}
	if len(got.Diffs) != 1 || got.Diffs[0].Method != "eth_getBalance" {
		t.Errorf("diffs not preserved: %+v", got.Diffs)
	}
	want := []string{"k1", "k2"}
	if !sort.StringsAreSorted(got.DoneKeys) {
		t.Errorf("done keys not sorted: %v", got.DoneKeys)
	}
	for i, k := range want {
		if got.DoneKeys[i] != k {
			t.Errorf("done_keys[%d] = %q, want %q", i, got.DoneKeys[i], k)
		}
	}
}

func TestLoadStateMissing(t *testing.T) {
	_, err := LoadState(filepath.Join(t.TempDir(), "missing.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestStateRecorderDedup(t *testing.T) {
	r := newStateRecorder("", &State{DoneKeys: []string{"a"}})
	if !r.isDone("a") {
		t.Fatal("expected a done from seed")
	}
	if r.isDone("b") {
		t.Fatal("b should not be done")
	}
	r.record("b", callOutcome{totalReqs: 1, successReqs: 1})
	if !r.isDone("b") {
		t.Fatal("b should be done after record")
	}
	// Recording the same key twice should not duplicate it in DoneKeys.
	r.record("b", callOutcome{totalReqs: 1, successReqs: 1})
	count := 0
	for _, k := range r.state.DoneKeys {
		if k == "b" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected b once in DoneKeys, got %d", count)
	}
	if r.state.TotalRequests != 2 {
		t.Errorf("counters should accumulate, got %d", r.state.TotalRequests)
	}
}

func TestStateRecorderFlush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	r := newStateRecorder(path, &State{EndpointA: "a", EndpointB: "b"})
	r.record("k1", callOutcome{totalReqs: 1, successReqs: 1})
	if err := r.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.EndpointA != "a" || len(s.DoneKeys) != 1 || s.DoneKeys[0] != "k1" {
		t.Errorf("unexpected state on disk: %+v", s)
	}
}
