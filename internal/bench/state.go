package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

// stateSchemaVersion is the on-disk schema version for bench state files.
const stateSchemaVersion = 1

// EndpointState is the durable per-endpoint snapshot used for crash-resume.
type EndpointState struct {
	Endpoint  string                  `json:"endpoint"`
	Total     int                     `json:"total"`
	Errors    int                     `json:"errors"`
	MinNs     int64                   `json:"min_ns"`
	MaxNs     int64                   `json:"max_ns"`
	StartTime time.Time               `json:"start_time"`
	HDR       *hdrhistogram.Snapshot  `json:"hdr,omitempty"`
}

// State is the bench resume state. Mode/Method/Params bind it to a specific
// invocation so we can reject mismatched resumes.
type State struct {
	SchemaVersion int             `json:"schema_version"`
	Mode          string          `json:"mode"` // currently always "requests-single-method"
	Method        string          `json:"method"`
	ParamsJSON    string          `json:"params_json"`
	Endpoints     []string        `json:"endpoints"`
	TargetTotal   int             `json:"target_total"`
	Per           []EndpointState `json:"per_endpoint"`
}

// LoadState reads a bench State from path.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse bench state %s: %w", path, err)
	}
	if s.SchemaVersion > stateSchemaVersion {
		return nil, fmt.Errorf("bench state %s schema_version=%d, this binary supports up to %d",
			path, s.SchemaVersion, stateSchemaVersion)
	}
	return &s, nil
}

// SaveState atomically writes s to path.
func SaveState(path string, s *State) error {
	if path == "" {
		return nil
	}
	s.SchemaVersion = stateSchemaVersion
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bench state: %w", err)
	}
	tmp := path + ".tmp"
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write bench state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename bench state: %w", err)
	}
	return nil
}

// Snapshot serializes m into an EndpointState suitable for persistence.
func (m *Metrics) Snapshot() EndpointState {
	es := EndpointState{
		Endpoint:  m.Endpoint,
		Total:     m.Total,
		Errors:    m.Errors,
		MinNs:     int64(m.min),
		MaxNs:     int64(m.max),
		StartTime: m.StartTime,
	}
	if m.hist != nil {
		es.HDR = m.hist.Export()
	}
	return es
}

// RestoreFromSnapshot replaces m's state with es. The HDR histogram is
// re-imported with the same bounds; counters and min/max are restored.
func (m *Metrics) RestoreFromSnapshot(es EndpointState) {
	m.Total = es.Total
	m.Errors = es.Errors
	m.min = time.Duration(es.MinNs)
	m.max = time.Duration(es.MaxNs)
	if !es.StartTime.IsZero() {
		m.StartTime = es.StartTime
	}
	if es.HDR != nil {
		m.hist = hdrhistogram.Import(es.HDR)
	}
}
