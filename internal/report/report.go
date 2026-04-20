// Package report handles output formatting for rpcduel.
package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/diff"
)

// Format selects the output format.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// DiffReport is the output for the diff command.
type DiffReport struct {
	Endpoints []string          `json:"endpoints"`
	Method    string            `json:"method"`
	Total     int               `json:"total"`
	DiffCount int               `json:"diff_count"`
	Diffs     []diff.Difference `json:"diffs,omitempty"`
}

// BenchReport is the output for the bench command.
type BenchReport struct {
	Summaries []bench.Summary `json:"summaries"`
}

// CallError describes a failed RPC call.
type CallError struct {
	Type    string `json:"type"`
	Code    *int   `json:"code,omitempty"`
	Message string `json:"message"`
}

// CallReport is the output for the call command.
type CallReport struct {
	Endpoint  string          `json:"endpoint"`
	Method    string          `json:"method"`
	Params    []interface{}   `json:"params"`
	Success   bool            `json:"success"`
	LatencyMS float64         `json:"latency_ms"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *CallError      `json:"error,omitempty"`
}

// DuelReport combines diff and bench results.
type DuelReport struct {
	Endpoints []string          `json:"endpoints"`
	Method    string            `json:"method"`
	Total     int               `json:"total"`
	DiffCount int               `json:"diff_count"`
	DiffRate  float64           `json:"diff_rate"`
	Diffs     []diff.Difference `json:"diffs,omitempty"`
	Metrics   []bench.Summary   `json:"metrics"`
}

// PrintDiff writes a diff report to the given writer.
func PrintDiff(w io.Writer, r DiffReport, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	fmt.Fprintf(w, "\nRPC Diff Result\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 40))
	fmt.Fprintf(w, "Method:     %s\n", r.Method)
	fmt.Fprintf(w, "Endpoints:  %s\n", strings.Join(r.Endpoints, " vs "))
	fmt.Fprintf(w, "Requests:   %d\n", r.Total)
	fmt.Fprintf(w, "Diffs:      %d\n", r.DiffCount)
	if len(r.Diffs) > 0 {
		fmt.Fprintf(w, "\nDifferences:\n")
		for _, d := range r.Diffs {
			fmt.Fprintf(w, "  %s\n", d.String())
		}
	} else {
		fmt.Fprintf(w, "\nNo differences found.\n")
	}
}

// PrintBench writes a bench report to the given writer.
func PrintBench(w io.Writer, r BenchReport, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	fmt.Fprintf(w, "\nBenchmark Result\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 40))
	for _, s := range r.Summaries {
		fmt.Fprintf(w, "Endpoint:   %s\n", s.Endpoint)
		if s.Scenario != "" {
			fmt.Fprintf(w, "  Scenario: %s\n", s.Scenario)
		}
		fmt.Fprintf(w, "  Requests: %d\n", s.Total)
		fmt.Fprintf(w, "  Errors:   %d (%.1f%%)\n", s.Errors, s.ErrorRate*100)
		fmt.Fprintf(w, "  QPS:      %.2f\n", s.QPS)
		fmt.Fprintf(w, "  Avg:      %s\n", s.AvgLatency)
		if s.P50 > 0 {
			fmt.Fprintf(w, "  P50:      %s\n", s.P50)
		}
		fmt.Fprintf(w, "  P95:      %s\n", s.P95)
		fmt.Fprintf(w, "  P99:      %s\n", s.P99)
		if s.P999 > 0 {
			fmt.Fprintf(w, "  P999:     %s\n", s.P999)
		}
		if s.Min > 0 || s.Max > 0 {
			fmt.Fprintf(w, "  Min:      %s\n", s.Min)
			fmt.Fprintf(w, "  Max:      %s\n", s.Max)
		}
		fmt.Fprintln(w)
	}
}

// PrintCall writes a single RPC call report to the given writer.
func PrintCall(w io.Writer, r CallReport, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}

	fmt.Fprintf(w, "\nRPC Call Result\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 40))
	fmt.Fprintf(w, "Endpoint: %s\n", r.Endpoint)
	fmt.Fprintf(w, "Method:   %s\n", r.Method)
	if !math.IsNaN(r.LatencyMS) && !math.IsInf(r.LatencyMS, 0) && r.LatencyMS > 0 {
		fmt.Fprintf(w, "Latency:  %.3fms\n", r.LatencyMS)
	}

	if params := formatJSON(r.Params); params != "" {
		fmt.Fprintf(w, "Params:\n%s\n", indentBlock(params, "  "))
	}

	if r.Error != nil {
		fmt.Fprintf(w, "Error:\n")
		if r.Error.Code != nil {
			fmt.Fprintf(w, "  [%s] code=%d message=%s\n", r.Error.Type, *r.Error.Code, r.Error.Message)
		} else {
			fmt.Fprintf(w, "  [%s] %s\n", r.Error.Type, r.Error.Message)
		}
		return
	}

	if result := formatRawJSON(r.Result); result != "" {
		fmt.Fprintf(w, "Result:\n%s\n", indentBlock(result, "  "))
	}
}

// WriteBenchCSV writes a detailed per-scenario benchmark report to w as CSV.
// Columns: endpoint, scenario, total, errors, error_rate_pct, qps,
// avg_latency_ms, p50_latency_ms, p95_latency_ms, p99_latency_ms,
// min_latency_ms, max_latency_ms.
func WriteBenchCSV(w io.Writer, summaries []bench.Summary) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"endpoint", "scenario", "total", "errors",
		"error_rate_pct", "qps",
		"avg_latency_ms", "p50_latency_ms", "p95_latency_ms", "p99_latency_ms", "p999_latency_ms",
		"min_latency_ms", "max_latency_ms",
	}); err != nil {
		return err
	}
	ms := float64(time.Millisecond)
	for _, s := range summaries {
		if err := cw.Write([]string{
			s.Endpoint,
			s.Scenario,
			strconv.Itoa(s.Total),
			strconv.Itoa(s.Errors),
			fmt.Sprintf("%.2f", s.ErrorRate*100),
			fmt.Sprintf("%.2f", s.QPS),
			fmt.Sprintf("%.3f", float64(s.AvgLatency)/ms),
			fmt.Sprintf("%.3f", float64(s.P50)/ms),
			fmt.Sprintf("%.3f", float64(s.P95)/ms),
			fmt.Sprintf("%.3f", float64(s.P99)/ms),
			fmt.Sprintf("%.3f", float64(s.P999)/ms),
			fmt.Sprintf("%.3f", float64(s.Min)/ms),
			fmt.Sprintf("%.3f", float64(s.Max)/ms),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// PrintDuel writes a duel report to the given writer.
func PrintDuel(w io.Writer, r DuelReport, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	fmt.Fprintf(w, "\nRPC Duel Result\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 40))
	fmt.Fprintf(w, "Method:     %s\n", r.Method)
	fmt.Fprintf(w, "Endpoints:  %s\n", strings.Join(r.Endpoints, " vs "))
	fmt.Fprintf(w, "Requests:   %d\n", r.Total)
	fmt.Fprintf(w, "Diffs:      %d (%.2f%%)\n", r.DiffCount, r.DiffRate*100)

	if len(r.Metrics) > 0 {
		fmt.Fprintf(w, "\nLatency:\n")
		for _, s := range r.Metrics {
			fmt.Fprintf(w, "  %s: avg %s, p95 %s, p99 %s, qps %.2f, errors %d\n",
				s.Endpoint, s.AvgLatency, s.P95, s.P99, s.QPS, s.Errors)
		}
	}

	if len(r.Diffs) > 0 {
		fmt.Fprintf(w, "\nSample Diffs (up to 5):\n")
		limit := 5
		if len(r.Diffs) < limit {
			limit = len(r.Diffs)
		}
		for _, d := range r.Diffs[:limit] {
			fmt.Fprintf(w, "  %s\n", d.String())
		}
	}
}

func formatJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func formatRawJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err == nil {
		return buf.String()
	}
	return string(raw)
}

func indentBlock(s, prefix string) string {
	if s == "" {
		return ""
	}
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
