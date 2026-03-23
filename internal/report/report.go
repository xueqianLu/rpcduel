// Package report handles output formatting for rpcduel.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
	Endpoints []string         `json:"endpoints"`
	Method    string           `json:"method"`
	Total     int              `json:"total"`
	DiffCount int              `json:"diff_count"`
	Diffs     []diff.Difference `json:"diffs,omitempty"`
}

// BenchReport is the output for the bench command.
type BenchReport struct {
	Summaries []bench.Summary `json:"summaries"`
}

// DuelReport combines diff and bench results.
type DuelReport struct {
	Endpoints []string         `json:"endpoints"`
	Method    string           `json:"method"`
	Total     int              `json:"total"`
	DiffCount int              `json:"diff_count"`
	DiffRate  float64          `json:"diff_rate"`
	Diffs     []diff.Difference `json:"diffs,omitempty"`
	Metrics   []bench.Summary  `json:"metrics"`
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
		fmt.Fprintf(w, "  Requests: %d\n", s.Total)
		fmt.Fprintf(w, "  Errors:   %d (%.1f%%)\n", s.Errors, s.ErrorRate*100)
		fmt.Fprintf(w, "  QPS:      %.2f\n", s.QPS)
		fmt.Fprintf(w, "  Avg:      %s\n", s.AvgLatency)
		fmt.Fprintf(w, "  P95:      %s\n", s.P95)
		fmt.Fprintf(w, "  P99:      %s\n", s.P99)
		fmt.Fprintln(w)
	}
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
