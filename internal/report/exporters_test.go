package report

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/replay"
	"github.com/xueqianLu/rpcduel/internal/thresholds"
)

func sampleSummaries() []bench.Summary {
	return []bench.Summary{{
		Endpoint: "https://a.example", Scenario: "eth_blockNumber",
		Total: 100, Errors: 5, ErrorRate: 0.05, QPS: 42,
		AvgLatency: 50 * time.Millisecond,
		P50:        40 * time.Millisecond,
		P95:        80 * time.Millisecond,
		P99:        120 * time.Millisecond,
		P999:       150 * time.Millisecond,
	}}
}

func TestWriteBenchHTML(t *testing.T) {
	var buf bytes.Buffer
	br := []thresholds.Breach{{Endpoint: "https://a.example", Metric: "p99_ms", Limit: 100, Actual: 120}}
	if err := WriteBenchHTML(&buf, BenchReport{Summaries: sampleSummaries()}, br, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"<!doctype html>", "FAIL", "https://a.example", "p99_ms"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestWriteBenchMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteBenchMarkdown(&buf, BenchReport{Summaries: sampleSummaries()}, nil, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "PASS") || !strings.Contains(out, "| Endpoint |") {
		t.Errorf("markdown unexpected: %s", out)
	}
}

func TestWriteBenchJUnit(t *testing.T) {
	var buf bytes.Buffer
	br := []thresholds.Breach{{Endpoint: "https://a.example", Metric: "p99_ms", Limit: 100, Actual: 120}}
	if err := WriteBenchJUnit(&buf, BenchReport{Summaries: sampleSummaries()}, br); err != nil {
		t.Fatal(err)
	}
	var parsed junitSuites
	if err := xml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Suites) == 0 || parsed.Suites[0].Failures != 1 {
		t.Errorf("expected 1 failure, got %+v", parsed)
	}
}

func TestWriteDuelExporters(t *testing.T) {
	rep := DuelReport{
		Endpoints: []string{"a", "b"}, Method: "eth_blockNumber",
		Total: 100, DiffCount: 2, DiffRate: 0.02,
		Diffs:   []diff.Difference{{Path: "$.x", Left: "1", Right: "2", Reason: "value"}},
		Metrics: sampleSummaries(),
	}
	var buf bytes.Buffer
	if err := WriteDuelHTML(&buf, rep, nil, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "duel report") {
		t.Errorf("missing duel report header")
	}
	buf.Reset()
	if err := WriteDuelMarkdown(&buf, rep, nil, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "| Endpoint |") {
		t.Errorf("missing latency table")
	}
	buf.Reset()
	if err := WriteDuelJUnit(&buf, rep, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "diff_rate") {
		t.Errorf("missing diff_rate testcase")
	}
}

func TestWriteReplayExporters(t *testing.T) {
	r := &replay.Result{
		AccountsTested: 1, TransactionsTested: 2, BlocksTested: 3,
		TotalRequests: 10, SuccessRequests: 9,
		Diffs: []replay.FoundDiff{{Category: replay.CategoryTx, Method: "eth_getTransactionByHash", Detail: "x"}},
	}
	var buf bytes.Buffer
	if err := WriteReplayHTML(&buf, r, nil, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "replay report") {
		t.Errorf("missing replay header")
	}
	buf.Reset()
	if err := WriteReplayMarkdown(&buf, r, nil, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Total requests") {
		t.Errorf("missing replay markdown body")
	}
	buf.Reset()
	if err := WriteReplayJUnit(&buf, r, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "mismatch_rate") {
		t.Errorf("missing mismatch_rate testcase")
	}
}
