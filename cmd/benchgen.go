// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/benchgen"
	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/runner"
	"github.com/xueqianLu/rpcduel/internal/tracerflag"
)

var benchgenCmd = &cobra.Command{
	Use:   "benchgen",
	Short: "Generate load-test scenarios from a dataset and run them directly",
	Long: `Load a dataset file (created with rpcduel dataset), generate weighted
RPC scenarios, optionally save them as a bench file, execute them against one
or more endpoints, and print a performance report. A detailed CSV report can
optionally be written to a file.

Scenarios generated (basic):
  balance, transaction_count, transaction_by_hash, transaction_receipt, block_by_number

Scenarios generated (complex):
  get_logs, mixed_balance

Optional trace scenarios:
  debug_trace_transaction, debug_trace_block`,
	RunE: runBenchgen,
}

var (
	benchgenDataset     string
	benchgenRPCs        []string
	benchgenConcurrency int
	benchgenRequests    int
	benchgenDuration    time.Duration
	benchgenTimeout     time.Duration
	benchgenTraceTx     bool
	benchgenTraceBlock  bool
	benchgenTracer      string
	benchgenTracerCfg   string
	benchgenOnly        []string
	benchgenOut         string
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
	benchgenCmd.Flags().BoolVar(&benchgenTraceTx, "trace-transaction", false, "Include debug_traceTransaction scenarios")
	benchgenCmd.Flags().BoolVar(&benchgenTraceBlock, "trace-block", false, "Include debug_traceBlockByNumber scenarios")
	benchgenCmd.Flags().StringVar(&benchgenTracer, "tracer", tracerflag.Default, tracerflag.FlagUsage())
	benchgenCmd.Flags().StringVar(&benchgenTracerCfg, "tracer-config", "", tracerflag.ConfigFlagUsage())
	benchgenCmd.Flags().StringSliceVar(&benchgenOnly, "only", nil,
		"Only include selected scenario groups (e.g. balance,transaction,block,logs,mixed_balance,trace)")
	benchgenCmd.Flags().StringVar(&benchgenOut, "out", "", "Write the generated bench scenario file to this path")
	benchgenCmd.Flags().StringVar(&benchgenCSV, "csv", "", "Write detailed per-scenario CSV report to this file")
	benchgenCmd.Flags().StringVar(&benchgenOutput, "output", "text", "Output format: text or json")
}

func runBenchgen(_ *cobra.Command, _ []string) error {
	if err := validateOutputFormat(benchgenOutput); err != nil {
		return err
	}
	if len(benchgenRPCs) == 0 && benchgenOut == "" {
		return fmt.Errorf("at least one --rpc endpoint or --out is required")
	}
	if len(benchgenOnly) > 0 && (benchgenTraceTx || benchgenTraceBlock) {
		return fmt.Errorf("--only cannot be combined with --trace-transaction or --trace-block")
	}
	only, err := parseBenchgenOnlyTargets(benchgenOnly)
	if err != nil {
		return err
	}
	tracerCfg, err := tracerflag.Build(benchgenTracer, benchgenTracerCfg)
	if err != nil {
		return err
	}

	ds, err := dataset.Load(benchgenDataset)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}

	bf := benchgen.GenerateWithOptions(ds, nil, benchgen.Options{
		TraceTransaction: benchgenTraceTx,
		TraceBlock:       benchgenTraceBlock,
		Only:             only,
		TracerConfig:     tracerCfg,
	})

	if benchgenOut != "" {
		if err := benchgen.SaveBenchFile(benchgenOut, bf); err != nil {
			return fmt.Errorf("write bench file: %w", err)
		}
		slog.Info("bench scenario file written", "path", benchgenOut)
	}

	if len(benchgenRPCs) == 0 {
		return nil
	}
	if len(bf.FlattenRequests()) == 0 {
		return fmt.Errorf("no requests could be generated from the dataset")
	}
	workers := benchgenConcurrency
	if workers <= 0 {
		workers = 1
	}

	var (
		taggedReqs []benchgen.TaggedRequest
		resultCh   <-chan runner.Result
	)
	if benchgenDuration > 0 {
		samplers := make([]*benchgen.WeightedTaggedSampler, workers)
		for i := range samplers {
			samplers[i] = bf.NewWeightedTaggedSampler(rand.New(rand.NewSource(42 + int64(i))))
		}
		resultCh = runner.RunDurationGenerated(context.Background(), workers, benchgenDuration, benchgenTimeout,
			func(workerID, iteration int) runner.Task {
				tr := samplers[workerID].Next()
				return runner.Task{
					Endpoint: benchgenRPCs[(workerID+iteration)%len(benchgenRPCs)],
					Tag:      tr.Scenario,
					Method:   tr.Method,
					Params:   tr.Params,
				}
			})
	} else {
		n := benchgenRequests
		if n <= 0 {
			n = 1000
		}
		taggedReqs = bf.WeightedTaggedRequests(n, nil)
	}

	if benchgenDuration == 0 && len(taggedReqs) == 0 {
		return fmt.Errorf("no requests could be generated from the dataset")
	}

	totalScenarios := len(bf.Scenarios)
	totalRequests := len(taggedReqs)
	if benchgenDuration > 0 {
		totalRequests = 0
	}
	slog.Info("running benchgen",
		"scenarios", totalScenarios,
		"requests", totalRequests,
		"concurrency", benchgenConcurrency,
		"endpoints", len(benchgenRPCs))

	// Per-(endpoint, scenario) metrics map.
	// The key is "endpoint\x00scenario"; the NUL byte is used as a separator
	// because it cannot appear in either a URL or a scenario name.
	metricsMap := make(map[string]*bench.Metrics)
	runStart := time.Now()
	getMetrics := func(endpoint, scenario string) *bench.Metrics {
		key := endpoint + "\x00" + scenario
		m := metricsMap[key]
		if m == nil {
			m = bench.NewMetricsAt(endpoint, runStart)
			m.Scenario = scenario
			metricsMap[key] = m
		}
		return m
	}

	if benchgenDuration == 0 {
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
		resultCh = runner.Run(context.Background(), tasks, benchgenConcurrency, benchgenTimeout)
	}

	for res := range resultCh {
		getMetrics(res.Endpoint, res.Tag).Record(res.Latency, res.Err != nil)
	}

	// Finalize, collect, and sort summaries for deterministic output.
	summaries := make([]bench.Summary, 0, len(metricsMap))
	runEnd := time.Now()
	for _, m := range metricsMap {
		m.FinishAt(runEnd)
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
		defer func() { _ = f.Close() }()
		if err := report.WriteBenchCSV(f, summaries); err != nil {
			return fmt.Errorf("write CSV report: %w", err)
		}
		slog.Info("CSV report written", "path", benchgenCSV)
	}
	return nil
}
