package bench_test

import (
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
)

func TestMetrics_Empty(t *testing.T) {
	m := bench.NewMetrics("http://localhost:8545")
	m.Finish()
	s := m.Summarize()
	if s.Total != 0 {
		t.Errorf("expected total 0, got %d", s.Total)
	}
	if s.QPS != 0 {
		t.Errorf("expected qps 0, got %f", s.QPS)
	}
}

func TestMetrics_Basic(t *testing.T) {
	m := bench.NewMetrics("http://localhost:8545")
	m.Record(100*time.Millisecond, false)
	m.Record(200*time.Millisecond, false)
	m.Record(300*time.Millisecond, true)

	s := m.Summarize()
	if s.Total != 3 {
		t.Errorf("expected total 3, got %d", s.Total)
	}
	if s.Errors != 1 {
		t.Errorf("expected 1 error, got %d", s.Errors)
	}
	// ErrorRate should be ~0.333
	if s.ErrorRate < 0.3 || s.ErrorRate > 0.4 {
		t.Errorf("unexpected error rate %f", s.ErrorRate)
	}
}

func TestMetrics_Percentile(t *testing.T) {
	m := bench.NewMetrics("http://ep")
	for i := 1; i <= 100; i++ {
		m.Record(time.Duration(i)*time.Millisecond, false)
	}
	m.Finish()
	p95 := m.Percentile(95)
	if p95 < 94*time.Millisecond || p95 > 96*time.Millisecond {
		t.Errorf("unexpected P95 %v", p95)
	}
	p99 := m.Percentile(99)
	if p99 < 98*time.Millisecond || p99 > 100*time.Millisecond {
		t.Errorf("unexpected P99 %v", p99)
	}
}

func TestMetrics_AvgLatency(t *testing.T) {
	m := bench.NewMetrics("http://ep")
	m.Record(100*time.Millisecond, false)
	m.Record(300*time.Millisecond, false)
	m.Finish()
	avg := m.AvgLatency()
	if avg != 200*time.Millisecond {
		t.Errorf("expected avg 200ms, got %v", avg)
	}
}

func TestMetrics_MinMax(t *testing.T) {
	m := bench.NewMetrics("http://ep")
	m.Record(50*time.Millisecond, false)
	m.Record(200*time.Millisecond, false)
	m.Record(100*time.Millisecond, false)
	m.Finish()
	s := m.Summarize()
	if s.Min != 50*time.Millisecond {
		t.Errorf("expected min 50ms, got %v", s.Min)
	}
	if s.Max != 200*time.Millisecond {
		t.Errorf("expected max 200ms, got %v", s.Max)
	}
}

func TestMetrics_Scenario(t *testing.T) {
	m := bench.NewMetrics("http://ep")
	m.Scenario = "balance"
	m.Record(10*time.Millisecond, false)
	m.Finish()
	s := m.Summarize()
	if s.Scenario != "balance" {
		t.Errorf("expected scenario 'balance', got %q", s.Scenario)
	}
}
