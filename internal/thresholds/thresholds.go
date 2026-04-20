// Package thresholds evaluates SLO breaches against rpcduel command
// reports and returns human-readable failure descriptions.
//
// A breach list is empty when every configured threshold passed (or
// when no thresholds were configured at all). Callers turn a non-empty
// breach list into a non-zero process exit code so rpcduel can be used
// as a CI gate.
package thresholds

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/config"
	"github.com/xueqianLu/rpcduel/internal/replay"
)

// Breach describes a single threshold violation.
type Breach struct {
	Endpoint string  // empty for aggregate metrics
	Metric   string  // e.g. "p99_ms", "error_rate"
	Limit    float64 // configured threshold
	Actual   float64 // observed value
}

// String renders the breach as "endpoint metric=actual > limit".
func (b Breach) String() string {
	prefix := ""
	if b.Endpoint != "" {
		prefix = b.Endpoint + " "
	}
	return fmt.Sprintf("%s%s actual=%.4g limit=%.4g", prefix, b.Metric, b.Actual, b.Limit)
}

// EvalBench checks every endpoint summary against t. min_qps is treated
// as a "lower bound" check; everything else is an upper bound.
func EvalBench(summaries []bench.Summary, t config.BenchThresholds) []Breach {
	var out []Breach
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	for _, s := range summaries {
		check := func(metric string, limit, actual float64, lower bool) {
			if limit <= 0 {
				return
			}
			if (lower && actual < limit) || (!lower && actual > limit) {
				out = append(out, Breach{Endpoint: s.Endpoint, Metric: metric, Limit: limit, Actual: actual})
			}
		}
		check("avg_ms", t.AvgMs, ms(s.AvgLatency), false)
		check("p50_ms", t.P50Ms, ms(s.P50), false)
		check("p95_ms", t.P95Ms, ms(s.P95), false)
		check("p99_ms", t.P99Ms, ms(s.P99), false)
		check("p999_ms", t.P999Ms, ms(s.P999), false)
		check("error_rate", t.ErrorRate, s.ErrorRate, false)
		check("min_qps", t.MinQPS, s.QPS, true)
	}
	return out
}

// EvalDuel checks the duel report's aggregate diff_rate plus per-endpoint
// latency/error thresholds.
func EvalDuel(diffRate float64, summaries []bench.Summary, t config.DuelThresholds) []Breach {
	var out []Breach
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	if t.DiffRate > 0 && diffRate > t.DiffRate {
		out = append(out, Breach{Metric: "diff_rate", Limit: t.DiffRate, Actual: diffRate})
	}
	for _, s := range summaries {
		check := func(metric string, limit, actual float64) {
			if limit <= 0 {
				return
			}
			if actual > limit {
				out = append(out, Breach{Endpoint: s.Endpoint, Metric: metric, Limit: limit, Actual: actual})
			}
		}
		check("p95_ms", t.P95Ms, ms(s.P95))
		check("p99_ms", t.P99Ms, ms(s.P99))
		check("error_rate", t.ErrorRate, s.ErrorRate)
	}
	return out
}

// EvalDiff checks the diff command's aggregate diff_count / diff_rate.
func EvalDiff(total, diffCount int, t config.DiffThresholds) []Breach {
	var out []Breach
	rate := 0.0
	if total > 0 {
		rate = float64(diffCount) / float64(total)
	}
	if t.DiffRate > 0 && rate > t.DiffRate {
		out = append(out, Breach{Metric: "diff_rate", Limit: t.DiffRate, Actual: rate})
	}
	if t.MaxDiffs > 0 && diffCount > t.MaxDiffs {
		out = append(out, Breach{Metric: "max_diffs", Limit: float64(t.MaxDiffs), Actual: float64(diffCount)})
	}
	return out
}

// EvalReplay checks the replay command's mismatch and error rates.
func EvalReplay(r *replay.Result, t config.ReplayThresholds) []Breach {
	var out []Breach
	if r == nil || r.TotalRequests == 0 {
		return nil
	}
	mismatchCount := len(r.Diffs)
	mismatchRate := float64(mismatchCount) / float64(r.TotalRequests)
	errorRate := 1 - r.SuccessRate()
	if t.MismatchRate > 0 && mismatchRate > t.MismatchRate {
		out = append(out, Breach{Metric: "mismatch_rate", Limit: t.MismatchRate, Actual: mismatchRate})
	}
	if t.ErrorRate > 0 && errorRate > t.ErrorRate {
		out = append(out, Breach{Metric: "error_rate", Limit: t.ErrorRate, Actual: errorRate})
	}
	if t.MaxMismatch > 0 && mismatchCount > t.MaxMismatch {
		out = append(out, Breach{Metric: "max_mismatch", Limit: float64(t.MaxMismatch), Actual: float64(mismatchCount)})
	}
	return out
}

// Print writes a human-readable PASS/FAIL summary to w. Returns true
// when at least one threshold breach is reported. When configured is
// false (no thresholds set anywhere) Print writes nothing and returns
// false.
func Print(w io.Writer, breaches []Breach, configured bool) bool {
	if !configured {
		return false
	}
	if len(breaches) == 0 {
		fmt.Fprintln(w, "\nThresholds: PASS")
		return false
	}
	var b strings.Builder
	b.WriteString("\nThresholds: FAIL\n")
	for _, br := range breaches {
		b.WriteString("  - ")
		b.WriteString(br.String())
		b.WriteByte('\n')
	}
	_, _ = io.WriteString(w, b.String())
	return true
}

// AnyConfigured returns true when at least one numeric threshold field
// in t is non-zero.
func AnyConfiguredBench(t config.BenchThresholds) bool {
	return t.P50Ms > 0 || t.P95Ms > 0 || t.P99Ms > 0 || t.P999Ms > 0 ||
		t.AvgMs > 0 || t.ErrorRate > 0 || t.MinQPS > 0
}

// AnyConfiguredDuel mirrors AnyConfiguredBench for DuelThresholds.
func AnyConfiguredDuel(t config.DuelThresholds) bool {
	return t.P95Ms > 0 || t.P99Ms > 0 || t.ErrorRate > 0 || t.DiffRate > 0
}

// AnyConfiguredDiff mirrors AnyConfiguredBench for DiffThresholds.
func AnyConfiguredDiff(t config.DiffThresholds) bool {
	return t.DiffRate > 0 || t.MaxDiffs > 0
}

// AnyConfiguredReplay mirrors AnyConfiguredBench for ReplayThresholds.
func AnyConfiguredReplay(t config.ReplayThresholds) bool {
	return t.MismatchRate > 0 || t.ErrorRate > 0 || t.MaxMismatch > 0
}
