// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// stateSchemaVersion is the on-disk schema version for replay state files.
// Bump when the State struct shape changes in an incompatible way.
const stateSchemaVersion = 1

// State is the durable on-disk record used to resume an interrupted replay.
type State struct {
	SchemaVersion   int          `json:"schema_version"`
	EndpointA       string       `json:"endpoint_a"`
	EndpointB       string       `json:"endpoint_b"`
	DatasetPath     string       `json:"dataset_path,omitempty"`
	DoneKeys        []string     `json:"done_keys"`
	TotalRequests   int          `json:"total_requests"`
	SuccessRequests int          `json:"success_requests"`
	Unsupported     int          `json:"unsupported"`
	Diffs           []FoundDiff  `json:"diffs"`
}

// LoadState reads a State from the JSON file at path. Returns os.ErrNotExist
// (wrapped) when the file does not exist so callers can distinguish.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state %s: %w", path, err)
	}
	if s.SchemaVersion > stateSchemaVersion {
		return nil, fmt.Errorf("state %s schema_version=%d, this binary supports up to %d",
			path, s.SchemaVersion, stateSchemaVersion)
	}
	return &s, nil
}

// SaveState atomically writes s to the JSON file at path.
func SaveState(path string, s *State) error {
	if path == "" {
		return nil
	}
	s.SchemaVersion = stateSchemaVersion
	// Sort keys for stable diffs across saves.
	sort.Strings(s.DoneKeys)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !os.IsExist(err) {
		// Fall through; WriteFile will surface a clearer error if needed.
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

// stateRecorder collects state mutations safely from the result loop and
// flushes them to disk on demand. It is intentionally serial; replay's
// result loop already runs in a single goroutine.
type stateRecorder struct {
	mu       sync.Mutex
	path     string
	state    *State
	doneKeys map[string]struct{}
	dirty    int
}

func newStateRecorder(path string, seed *State) *stateRecorder {
	if seed == nil {
		seed = &State{SchemaVersion: stateSchemaVersion}
	}
	r := &stateRecorder{path: path, state: seed, doneKeys: make(map[string]struct{}, len(seed.DoneKeys))}
	for _, k := range seed.DoneKeys {
		r.doneKeys[k] = struct{}{}
	}
	return r
}

func (r *stateRecorder) isDone(key string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.doneKeys[key]
	return ok
}

func (r *stateRecorder) record(key string, out callOutcome) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.doneKeys[key]; !ok {
		r.doneKeys[key] = struct{}{}
		r.state.DoneKeys = append(r.state.DoneKeys, key)
	}
	r.state.TotalRequests += out.totalReqs
	r.state.SuccessRequests += out.successReqs
	r.state.Unsupported += out.unsupported
	if out.diff != nil {
		r.state.Diffs = append(r.state.Diffs, *out.diff)
	}
	r.dirty++
}

// flush writes the state to disk if non-empty path; resets dirty counter.
func (r *stateRecorder) flush() error {
	if r == nil || r.path == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirty = 0
	return SaveState(r.path, r.state)
}

func (r *stateRecorder) shouldFlush(every int) bool {
	if r == nil || every <= 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dirty >= every
}
