package report_test

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/report"
)

func makeSummaries() []bench.Summary {
	return []bench.Summary{
		{
			Endpoint:   "http://rpc-a.example.com",
			Scenario:   "balance",
			Total:      500,
			Errors:     5,
			ErrorRate:  0.01,
			QPS:        120.5,
			AvgLatency: 15 * time.Millisecond,
			P50:        12 * time.Millisecond,
			P95:        30 * time.Millisecond,
			P99:        50 * time.Millisecond,
			Min:        5 * time.Millisecond,
			Max:        80 * time.Millisecond,
		},
		{
			Endpoint:   "http://rpc-a.example.com",
			Scenario:   "transaction_by_hash",
			Total:      300,
			Errors:     0,
			ErrorRate:  0,
			QPS:        85.2,
			AvgLatency: 20 * time.Millisecond,
			P50:        18 * time.Millisecond,
			P95:        40 * time.Millisecond,
			P99:        60 * time.Millisecond,
			Min:        10 * time.Millisecond,
			Max:        90 * time.Millisecond,
		},
	}
}

func TestWriteBenchCSV_Header(t *testing.T) {
	var buf bytes.Buffer
	if err := report.WriteBenchCSV(&buf, makeSummaries()); err != nil {
		t.Fatalf("WriteBenchCSV: %v", err)
	}
	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least a header row")
	}
	header := records[0]
	expected := []string{
		"endpoint", "scenario", "total", "errors",
		"error_rate_pct", "qps",
		"avg_latency_ms", "p50_latency_ms", "p95_latency_ms", "p99_latency_ms",
		"min_latency_ms", "max_latency_ms",
	}
	if len(header) != len(expected) {
		t.Errorf("expected %d header columns, got %d: %v", len(expected), len(header), header)
	}
	for i, col := range expected {
		if i >= len(header) {
			break
		}
		if header[i] != col {
			t.Errorf("header[%d]: expected %q, got %q", i, col, header[i])
		}
	}
}

func TestWriteBenchCSV_Rows(t *testing.T) {
	sums := makeSummaries()
	var buf bytes.Buffer
	if err := report.WriteBenchCSV(&buf, sums); err != nil {
		t.Fatalf("WriteBenchCSV: %v", err)
	}
	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	// 1 header + 2 data rows.
	if len(records) != 3 {
		t.Errorf("expected 3 rows (header+2 data), got %d", len(records))
	}
	// Spot-check first data row.
	row := records[1]
	if row[0] != sums[0].Endpoint {
		t.Errorf("expected endpoint %q, got %q", sums[0].Endpoint, row[0])
	}
	if row[1] != sums[0].Scenario {
		t.Errorf("expected scenario %q, got %q", sums[0].Scenario, row[1])
	}
	if row[2] != "500" {
		t.Errorf("expected total '500', got %q", row[2])
	}
}

func TestWriteBenchCSV_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := report.WriteBenchCSV(&buf, nil); err != nil {
		t.Fatalf("WriteBenchCSV empty: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	// Only header should be present.
	if !strings.HasPrefix(out, "endpoint,") {
		t.Errorf("expected header-only output, got: %q", out)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line for empty summaries, got %d", len(lines))
	}
}

func TestPrintBench_ShowsScenario(t *testing.T) {
	rep := report.BenchReport{Summaries: makeSummaries()}
	var buf bytes.Buffer
	report.PrintBench(&buf, rep, report.FormatText)
	out := buf.String()
	if !strings.Contains(out, "balance") {
		t.Error("expected scenario 'balance' in text output")
	}
	if !strings.Contains(out, "Scenario:") {
		t.Error("expected 'Scenario:' label in text output")
	}
}
