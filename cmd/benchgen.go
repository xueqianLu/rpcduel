package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/benchgen"
	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/runner"
)

var benchgenCmd = &cobra.Command{
	Use:   "benchgen",
	Short: "Generate load-test scenarios from a dataset and run them directly",
	Long: `Load a dataset file (created with rpcduel dataset), generate weighted
RPC scenarios, execute them against one or more endpoints, and print a
performance report. A detailed CSV report can optionally be written to a file.

Scenarios generated (basic):
  balance, transaction_count, transaction_by_hash, transaction_receipt, block_by_number

Scenarios generated (complex):
  get_logs, debug_trace_transaction, debug_trace_block, mixed_balance`,
	RunE: runBenchgen,
}

var (
	benchgenDataset     string
	benchgenRPCs        []string
	benchgenConcurrency int
	benchgenRequests    int
	benchgenDuration    time.Duration
	benchgenTimeout     time.Duration
	benchgenCSV         string
	benchgenOutput      string
)

func init() {
	benchgenCmd.Flags().StringVar(&benchgenDataset, "dataset", "dataset.json", "Path to the dataset JSON file")
	benchgenCmd.Flags().StringArrayVar(&benchgenRPCs, "rpc", nil, "RPC endpoint URL (can be specified multiple times)")
	benchgenCmd.Flags().IntVar(&benchgenConcurrency, "concurrency", 10, "Number of concurrent workers")
	benchgenCmd.Flags().IntVar(&benchgenRequests, "requests", 1000, "Total requests to send (0 = use --duration)")
	benchgenCmd.Flags().DurationVar(&benchgenDuration, "duration", 0, "Run for this long instead of fixed request count (e.g. 30s)")
	benchgenCmd.Flags().DurationVar(&benchgenTimeout, "timeout", 30*time.Second, "Per-request timeout")
	benchgenCmd.Flags().StringVar(&benchgenCSV, "csv", "", "Write detailed per-scenario CSV report to this file")
	benchgenCmd.Flags().StringVar(&benchgenOutput, "output", "text", "Output format: text or json")
}

func runBenchgen(cmd *cobra.Command, args []string) error {
	if len(benchgenRPCs) == 0 {
		return fmt.Errorf("at least one --rpc endpoint is required")
	}

	ds, err := dataset.Load(benchgenDataset)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}

	bf := benchgen.Generate(ds, nil)

	// Build tagged request list weighted by scenario.
	var taggedReqs []benchgen.TaggedRequest
	if benchgenDuration > 0 {
		// For duration mode, build a large pool to cycle through.
		taggedReqs = bf.WeightedTaggedRequests(benchgenConcurrency*1000, nil)
	} else {
		n := benchgenRequests
		if n <= 0 {
			n = 1000
		}
		taggedReqs = bf.WeightedTaggedRequests(n, nil)
	}

	if len(taggedReqs) == 0 {
		return fmt.Errorf("no requests could be generated from the dataset")
	}

	totalScenarios := len(bf.Scenarios)
	totalRequests := len(taggedReqs)
	fmt.Fprintf(os.Stderr, "Running benchgen: scenarios=%d requests=%d concurrency=%d endpoints=%d\n",
		totalScenarios, totalRequests, benchgenConcurrency, len(benchgenRPCs))

	// Convert to runner tasks, round-robining across endpoints.
	tasks := make([]runner.Task, len(taggedReqs))
	for i, req := range taggedReqs {
		tasks[i] = runner.Task{
			Endpoint: benchgenRPCs[i%len(benchgenRPCs)],
			Tag:      req.Scenario,
			Method:   req.Method,
			Params:   req.Params,
		}
	}

	// Per-(endpoint, scenario) metrics map.
	// The key is "endpoint\x00scenario"; the NUL byte is used as a separator
	// because it cannot appear in either a URL or a scenario name.
	metricsMap := make(map[string]*bench.Metrics)
	getMetrics := func(endpoint, scenario string) *bench.Metrics {
		key := endpoint + "\x00" + scenario
		m := metricsMap[key]
		if m == nil {
			m = bench.NewMetrics(endpoint)
			m.Scenario = scenario
			metricsMap[key] = m
		}
		return m
	}

	ctx := context.Background()

	var resultCh <-chan runner.Result
	if benchgenDuration > 0 {
		resultCh = runner.RunDurationFromTasks(ctx, tasks, benchgenConcurrency, benchgenDuration, benchgenTimeout)
	} else {
		resultCh = runner.Run(ctx, tasks, benchgenConcurrency, benchgenTimeout)
	}

	for res := range resultCh {
		getMetrics(res.Endpoint, res.Tag).Record(res.Latency, res.Err != nil)
	}

	// Finalize, collect, and sort summaries for deterministic output.
	summaries := make([]bench.Summary, 0, len(metricsMap))
	for _, m := range metricsMap {
		m.Finish()
		summaries = append(summaries, m.Summarize())
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Scenario != summaries[j].Scenario {
			return summaries[i].Scenario < summaries[j].Scenario
		}
		return summaries[i].Endpoint < summaries[j].Endpoint
	})

	// Print summary to stdout.
	rep := report.BenchReport{Summaries: summaries}
	report.PrintBench(os.Stdout, rep, report.Format(benchgenOutput))

	// Write CSV if requested.
	if benchgenCSV != "" {
		f, err := os.Create(benchgenCSV)
		if err != nil {
			return fmt.Errorf("create CSV report: %w", err)
		}
		defer f.Close()
		if err := report.WriteBenchCSV(f, summaries); err != nil {
			return fmt.Errorf("write CSV report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "CSV report written to %s\n", benchgenCSV)
	}
	return nil
}
