package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/config"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/metrics"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/rpc"
	"github.com/xueqianLu/rpcduel/internal/runner"
	"github.com/xueqianLu/rpcduel/internal/thresholds"
)

var duelCmd = &cobra.Command{
	Use:   "duel",
	Short: "Benchmark and diff two RPC endpoints simultaneously",
	Long: `Run concurrent requests against two RPC endpoints, collecting both
performance metrics and response consistency (diff) statistics.`,
	RunE: runDuel,
}

var (
	duelRPCs         []string
	duelMethod       string
	duelParamsStr    string
	duelConcurrency  int
	duelRequests     int
	duelDuration     time.Duration
	duelTimeout      time.Duration
	duelOutput       string
	duelIgnoreFields []string
	duelIgnoreOrder  bool
	duelWarmup       time.Duration
	duelReportHTML   string
	duelReportMD     string
	duelReportJUnit  string
	duelTP99         float64
	duelTErrorRate   float64
	duelTDiffRate    float64
)

func init() {
	duelCmd.Flags().StringArrayVar(&duelRPCs, "rpc", nil, "RPC endpoint URL (specify exactly 2)")
	duelCmd.Flags().StringVar(&duelMethod, "method", "eth_blockNumber", "JSON-RPC method name")
	duelCmd.Flags().StringVar(&duelParamsStr, "params", "[]", "JSON-encoded params array")
	duelCmd.Flags().IntVar(&duelConcurrency, "concurrency", 10, "Number of concurrent workers")
	duelCmd.Flags().IntVar(&duelRequests, "requests", 100, "Total request pairs (0 = use duration)")
	duelCmd.Flags().DurationVar(&duelDuration, "duration", 0, "Run for this long instead of fixed count (e.g. 30s)")
	duelCmd.Flags().DurationVar(&duelTimeout, "timeout", 30*time.Second, "Per-request timeout")
	duelCmd.Flags().StringVar(&duelOutput, "output", "text", "Output format: text or json")
	duelCmd.Flags().StringArrayVar(&duelIgnoreFields, "ignore-field", nil, "JSON field names to ignore in comparison")
	duelCmd.Flags().BoolVar(&duelIgnoreOrder, "ignore-order", false, "Treat arrays as unordered sets")
	duelCmd.Flags().DurationVar(&duelWarmup, "warmup", 0, "Discard results from the first N (e.g. 5s) of the run, before measurement begins")
	duelCmd.Flags().StringVar(&duelReportHTML, "report-html", "", "Write a self-contained HTML report to this path")
	duelCmd.Flags().StringVar(&duelReportMD, "report-md", "", "Write a Markdown report to this path")
	duelCmd.Flags().StringVar(&duelReportJUnit, "report-junit", "", "Write a JUnit XML report to this path")
	duelCmd.Flags().Float64Var(&duelTP99, "max-p99-ms", 0, "Fail (exit 2) if either endpoint's P99 latency exceeds this many ms")
	duelCmd.Flags().Float64Var(&duelTErrorRate, "max-error-rate", 0, "Fail (exit 2) if either endpoint's error rate exceeds this fraction (0..1)")
	duelCmd.Flags().Float64Var(&duelTDiffRate, "max-diff-rate", 0, "Fail (exit 2) if the response-mismatch rate exceeds this fraction (0..1)")
}

// fillDuelDefaults applies the duel: section of the loaded config to
// any flag the user did not explicitly set.
func fillDuelDefaults(cmd *cobra.Command) {
	if globalConfig == nil {
		return
	}
	d := globalConfig.Duel
	f := cmd.Flags()
	if !f.Changed("method") && d.Method != "" {
		duelMethod = d.Method
	}
	if !f.Changed("params") && d.Params != "" {
		duelParamsStr = d.Params
	}
	if !f.Changed("concurrency") && d.Concurrency > 0 {
		duelConcurrency = d.Concurrency
	}
	if !f.Changed("requests") && d.Requests > 0 {
		duelRequests = d.Requests
	}
	if !f.Changed("duration") && d.Duration > 0 {
		duelDuration = d.Duration
	}
	if !f.Changed("timeout") && d.Timeout > 0 {
		duelTimeout = d.Timeout
	}
	if !f.Changed("warmup") && d.Warmup > 0 {
		duelWarmup = d.Warmup
	}
	if !f.Changed("output") && d.Output != "" {
		duelOutput = d.Output
	}
	if !f.Changed("ignore-field") && len(d.IgnoreFields) > 0 {
		duelIgnoreFields = append([]string{}, d.IgnoreFields...)
	}
	if !f.Changed("ignore-order") && d.IgnoreOrder {
		duelIgnoreOrder = true
	}
}

func runDuel(cmd *cobra.Command, args []string) error {
	fillDuelDefaults(cmd)
	if err := validateOutputFormat(duelOutput); err != nil {
		return err
	}
	duelRPCs = rpcsFromConfig(duelRPCs)
	if len(duelRPCs) != 2 {
		return fmt.Errorf("exactly 2 --rpc endpoints are required")
	}

	epA := duelRPCs[0]
	epB := duelRPCs[1]

	params, err := rpc.ParseParams(duelParamsStr)
	if err != nil {
		return err
	}

	opts := diff.DefaultOptions()
	for _, f := range duelIgnoreFields {
		opts.IgnoreFields[f] = true
	}
	opts.IgnoreOrder = duelIgnoreOrder

	ctx := runnerContext(context.Background())
	outFmt := report.Format(duelOutput)

	startTime := time.Now()
	warmupEnd := startTime.Add(duelWarmup)
	if duelWarmup > 0 {
		slog.Info("warmup phase started", "duration", duelWarmup)
	}

	metricsA := bench.NewMetricsAt(epA, warmupEnd)
	metricsB := bench.NewMetricsAt(epB, warmupEnd)
	var allDiffs []diff.Difference
	total := 0

	var pairCh <-chan runner.PairResult

	if duelDuration > 0 {
		pairCh = runner.PairResultFromDuration(ctx, epA, epB, duelMethod, params,
			duelConcurrency, duelDuration, duelTimeout)
	} else {
		if duelRequests <= 0 {
			duelRequests = 100
		}
		pairCh = runner.RunPaired(ctx, epA, epB, duelMethod, params,
			duelConcurrency, duelRequests, duelTimeout)
	}

	for pair := range pairCh {
		// Use the later of the two timestamps so warmup-window pairs are
		// fully discarded even if one leg returned early.
		pairTs := pair.Left.Timestamp
		if pair.Right.Timestamp.After(pairTs) {
			pairTs = pair.Right.Timestamp
		}
		if duelWarmup > 0 && pairTs.Before(warmupEnd) {
			continue
		}
		total++
		metricsA.Record(pair.Left.Latency, pair.Left.Err != nil)
		metricsB.Record(pair.Right.Latency, pair.Right.Err != nil)
		scenario := scenarioLabel(pair.Left.Tag, duelMethod)
		metrics.Observe(epA, scenario, pair.Left.Latency, pair.Left.Err != nil)
		metrics.Observe(epB, scenario, pair.Right.Latency, pair.Right.Err != nil)

		if pair.Left.Err != nil || pair.Right.Err != nil {
			if pair.Left.Err == nil || pair.Right.Err == nil {
				allDiffs = append(allDiffs, diff.Difference{
					Path:   "$",
					Left:   fmt.Sprintf("%v", pair.Left.Err),
					Right:  fmt.Sprintf("%v", pair.Right.Err),
					Reason: "one endpoint errored",
				})
			}
			continue
		}

		lJSON := runner.ResultToJSON(pair.Left.Response)
		rJSON := runner.ResultToJSON(pair.Right.Response)
		diffs, err := diff.Compare(lJSON, rJSON, opts)
		if err != nil {
			slog.Warn("compare error", "err", err)
			continue
		}
		allDiffs = append(allDiffs, diffs...)
		metrics.ObserveDiff(epA, epB, len(diffs))
	}

	diffRate := 0.0
	if total > 0 {
		diffRate = float64(len(allDiffs)) / float64(total)
	}

	metricsA.Finish()
	metricsB.Finish()
	sumA := metricsA.Summarize()
	sumB := metricsB.Summarize()

	rep := report.DuelReport{
		Endpoints: []string{epA, epB},
		Method:    duelMethod,
		Total:     total,
		DiffCount: len(allDiffs),
		DiffRate:  diffRate,
		Diffs:     allDiffs,
		Metrics:   []bench.Summary{sumA, sumB},
	}
	report.PrintDuel(os.Stdout, rep, outFmt)

	t := config.DuelThresholds{}
	if globalConfig != nil {
		t = globalConfig.Thresholds.Duel
	}
	if cmd.Flags().Changed("max-p99-ms") {
		t.P99Ms = duelTP99
	}
	if cmd.Flags().Changed("max-error-rate") {
		t.ErrorRate = duelTErrorRate
	}
	if cmd.Flags().Changed("max-diff-rate") {
		t.DiffRate = duelTDiffRate
	}
	configured := thresholds.AnyConfiguredDuel(t)
	breaches := thresholds.EvalDuel(diffRate, rep.Metrics, t)

	htmlOut, mdOut, junitOut := reportPaths(duelReportHTML, duelReportMD, duelReportJUnit)
	if err := writeFile(htmlOut, func(w *os.File) error { return report.WriteDuelHTML(w, rep, breaches, configured) }); err != nil {
		return err
	}
	if err := writeFile(mdOut, func(w *os.File) error { return report.WriteDuelMarkdown(w, rep, breaches, configured) }); err != nil {
		return err
	}
	if err := writeFile(junitOut, func(w *os.File) error { return report.WriteDuelJUnit(w, rep, breaches) }); err != nil {
		return err
	}

	emitBreaches(breaches, configured)
	return nil
}
