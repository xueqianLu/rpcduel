// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package bench provides benchmarking metrics collection.
package bench

import (
	"io"
	"time"

	hdrhistogram "github.com/HdrHistogram/hdrhistogram-go"
)

// Histogram bounds: 1µs .. 1 minute, 3 significant figures (~0.1% precision).
const (
	hdrMinValue       = 1
	hdrMaxValue       = int64(60 * time.Second / time.Microsecond) // 60s in µs
	hdrSignificantFig = 3
)

// Metrics collects latency and error statistics.
//
// Latencies are stored in an HDR histogram for O(1) memory and accurate
// high-percentile reporting (P999) without sorting full latency slices.
type Metrics struct {
	Endpoint  string
	Scenario  string // optional scenario label (set by caller)
	Total     int
	Errors    int
	StartTime time.Time
	EndTime   time.Time

	hist *hdrhistogram.Histogram
	min  time.Duration
	max  time.Duration
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
		hist:      hdrhistogram.New(hdrMinValue, hdrMaxValue, hdrSignificantFig),
	}
}

// Record adds a single request result.
func (m *Metrics) Record(latency time.Duration, isError bool) {
	m.Total++
	if isError {
		m.Errors++
	}
	v := int64(latency / time.Microsecond)
	if v < hdrMinValue {
		v = hdrMinValue
	}
	if v > hdrMaxValue {
		v = hdrMaxValue
	}
	_ = m.hist.RecordValue(v)
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
	if m.hist == nil || m.hist.TotalCount() == 0 {
		return 0
	}
	return time.Duration(m.hist.Mean()) * time.Microsecond
}

// Percentile returns the p-th percentile latency (e.g., 95 for P95).
func (m *Metrics) Percentile(p float64) time.Duration {
	if m.hist == nil || m.hist.TotalCount() == 0 {
		return 0
	}
	return time.Duration(m.hist.ValueAtQuantile(p)) * time.Microsecond
}

// WriteHDR writes the percentile distribution log (HDR Histogram text
// format) to w. Useful for hdrhistogram-go / wrk2 tooling.
func (m *Metrics) WriteHDR(w io.Writer) error {
	if m.hist == nil {
		return nil
	}
	_, err := m.hist.PercentilesPrint(w, 5, 1.0)
	return err
}

// Histogram returns the underlying HDR histogram (mainly for tests).
func (m *Metrics) Histogram() *hdrhistogram.Histogram { return m.hist }

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
	P999       time.Duration
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
		P999:       m.Percentile(99.9),
		Min:        m.min,
		Max:        m.max,
	}
}
