// Package bench provides benchmarking metrics collection.
package bench

import (
	"math"
	"sort"
	"time"
)

// Metrics collects latency and error statistics.
type Metrics struct {
	Endpoint  string
	Scenario  string // optional scenario label (set by caller)
	Total     int
	Errors    int
	Latencies []time.Duration
	min       time.Duration
	max       time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// NewMetrics creates a new Metrics for the given endpoint.
func NewMetrics(endpoint string) *Metrics {
	return NewMetricsAt(endpoint, time.Now())
}

// NewMetricsAt creates a new Metrics using an explicit benchmark start time.
func NewMetricsAt(endpoint string, start time.Time) *Metrics {
	return &Metrics{
		Endpoint:  endpoint,
		StartTime: start,
	}
}

// Record adds a single request result.
func (m *Metrics) Record(latency time.Duration, isError bool) {
	m.Total++
	m.Latencies = append(m.Latencies, latency)
	if isError {
		m.Errors++
	}
	if m.Total == 1 || latency < m.min {
		m.min = latency
	}
	if latency > m.max {
		m.max = latency
	}
}

// Finish marks the end of the benchmark.
func (m *Metrics) Finish() {
	m.FinishAt(time.Now())
}

// FinishAt marks the end of the benchmark using an explicit timestamp.
func (m *Metrics) FinishAt(end time.Time) {
	m.EndTime = end
}

// Duration returns the total elapsed time.
func (m *Metrics) Duration() time.Duration {
	return m.EndTime.Sub(m.StartTime)
}

// QPS returns requests per second.
func (m *Metrics) QPS() float64 {
	d := m.Duration().Seconds()
	if d == 0 {
		return 0
	}
	return float64(m.Total) / d
}

// ErrorRate returns the fraction of errored requests.
func (m *Metrics) ErrorRate() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.Errors) / float64(m.Total)
}

// AvgLatency returns the mean latency.
func (m *Metrics) AvgLatency() time.Duration {
	if len(m.Latencies) == 0 {
		return 0
	}
	var sum time.Duration
	for _, l := range m.Latencies {
		sum += l
	}
	return sum / time.Duration(len(m.Latencies))
}

// Percentile returns the p-th percentile latency (e.g., 95 for P95).
func (m *Metrics) Percentile(p float64) time.Duration {
	if len(m.Latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(m.Latencies))
	copy(sorted, m.Latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Ceil(p/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Summary is a snapshot of computed metrics.
type Summary struct {
	Endpoint   string
	Scenario   string // non-empty when per-scenario tracking is used
	Total      int
	Errors     int
	ErrorRate  float64
	QPS        float64
	AvgLatency time.Duration
	P50        time.Duration
	P95        time.Duration
	P99        time.Duration
	Min        time.Duration
	Max        time.Duration
}

// Summarize computes the summary from the collected metrics.
// Call Finish() before Summarize() to set the end time.
func (m *Metrics) Summarize() Summary {
	return Summary{
		Endpoint:   m.Endpoint,
		Scenario:   m.Scenario,
		Total:      m.Total,
		Errors:     m.Errors,
		ErrorRate:  m.ErrorRate(),
		QPS:        m.QPS(),
		AvgLatency: m.AvgLatency(),
		P50:        m.Percentile(50),
		P95:        m.Percentile(95),
		P99:        m.Percentile(99),
		Min:        m.min,
		Max:        m.max,
	}
}
