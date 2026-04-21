package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/config"
	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/metrics"
	"github.com/xueqianLu/rpcduel/internal/replay"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/thresholds"
)

var diffTestCmd = &cobra.Command{
	Use:     "replay",
	Aliases: []string{"diff-test"},
	Short:   "Replay dataset-backed RPC calls across two endpoints and compare the results",
	Long: `Load a dataset file (created with rpcduel dataset) and execute RPC calls
for each account, transaction, and block against two endpoints, reporting any
response differences.`,
	RunE: runDiffTest,
}

var (
	diffTestDataset          string
	diffTestRPCs             []string
	diffTestMaxTxPerAccount  int
	diffTestTraceTransaction bool
	diffTestTraceBlock       bool
	diffTestOnly             []string
	diffTestOutput           string
	diffTestIgnoreFields     []string
	diffTestTimeout          time.Duration
	diffTestConcurrency      int
	diffTestReport           string
	diffTestCSV              string
	diffTestReportHTML       string
	diffTestReportMD         string
	diffTestReportJUnit      string
	diffTestTMismatchRate    float64
	diffTestTErrorRate       float64
	diffTestTMaxMismatch     int
	diffTestStateFile        string
	diffTestResume           bool
	diffTestStateInterval    int
)

func init() {
	diffTestCmd.Flags().StringVar(&diffTestDataset, "dataset", "dataset.json", "Path to the dataset JSON file")
	diffTestCmd.Flags().StringArrayVar(&diffTestRPCs, "rpc", nil, "RPC endpoint URL (specify exactly 2)")
	diffTestCmd.Flags().IntVar(&diffTestMaxTxPerAccount, "max-tx-per-account", 100,
		"Maximum transactions to test per account (0 = unlimited)")
	diffTestCmd.Flags().BoolVar(&diffTestTraceTransaction, "trace-transaction", false,
		"Also compare debug_traceTransaction for dataset transactions")
	diffTestCmd.Flags().BoolVar(&diffTestTraceBlock, "trace-block", false,
		"Also compare debug_traceBlockByNumber for dataset blocks")
	diffTestCmd.Flags().StringSliceVar(&diffTestOnly, "only", nil,
		"Only run selected replay targets (e.g. balance,transaction,block,trace,trace_transaction,trace_block)")
	diffTestCmd.Flags().StringVar(&diffTestOutput, "output", "text", "Output format: text or json")
	diffTestCmd.Flags().StringArrayVar(&diffTestIgnoreFields, "ignore-field", nil,
		"JSON field names to ignore in comparison")
	diffTestCmd.Flags().DurationVar(&diffTestTimeout, "timeout", 30*time.Second, "Per-request timeout")
	diffTestCmd.Flags().IntVar(&diffTestConcurrency, "concurrency", 4, "Number of goroutines used to execute RPC calls")
	diffTestCmd.Flags().StringVar(&diffTestReport, "report", "", "Write the report to this file (in addition to stdout)")
	diffTestCmd.Flags().StringVar(&diffTestCSV, "csv", "", "Write a CSV report of all diffs to this file")
	diffTestCmd.Flags().StringVar(&diffTestReportHTML, "report-html", "", "Write a self-contained HTML report to this path")
	diffTestCmd.Flags().StringVar(&diffTestReportMD, "report-md", "", "Write a Markdown report to this path")
	diffTestCmd.Flags().StringVar(&diffTestReportJUnit, "report-junit", "", "Write a JUnit XML report to this path")
	diffTestCmd.Flags().Float64Var(&diffTestTMismatchRate, "max-mismatch-rate", 0, "Fail (exit 2) if mismatch rate exceeds this fraction (0..1)")
	diffTestCmd.Flags().Float64Var(&diffTestTErrorRate, "max-error-rate", 0, "Fail (exit 2) if RPC error rate exceeds this fraction (0..1)")
	diffTestCmd.Flags().IntVar(&diffTestTMaxMismatch, "max-mismatch", 0, "Fail (exit 2) if more than this many mismatches are observed")
	diffTestCmd.Flags().StringVar(&diffTestStateFile, "state-file", "", "Path to a state file used for crash-resume; written periodically and on Ctrl+C")
	diffTestCmd.Flags().BoolVar(&diffTestResume, "resume", false, "Resume a previous run from --state-file (skips already-completed task keys)")
	diffTestCmd.Flags().IntVar(&diffTestStateInterval, "state-interval", 100, "Flush the state file every N completed tasks")
}

// fillReplayDefaults applies the replay: section of the loaded config.
func fillReplayDefaults(cmd *cobra.Command) {
	if globalConfig == nil {
		return
	}
	r := globalConfig.Replay
	f := cmd.Flags()
	if !f.Changed("dataset") && r.Dataset != "" {
		diffTestDataset = r.Dataset
	}
	if !f.Changed("max-tx-per-account") && r.MaxTxPerAccount > 0 {
		diffTestMaxTxPerAccount = r.MaxTxPerAccount
	}
	if !f.Changed("trace-transaction") && r.TraceTransaction {
		diffTestTraceTransaction = true
	}
	if !f.Changed("trace-block") && r.TraceBlock {
		diffTestTraceBlock = true
	}
	if !f.Changed("only") && len(r.Only) > 0 {
		diffTestOnly = append([]string{}, r.Only...)
	}
	if !f.Changed("ignore-field") && len(r.IgnoreFields) > 0 {
		diffTestIgnoreFields = append([]string{}, r.IgnoreFields...)
	}
	if !f.Changed("timeout") && r.Timeout > 0 {
		diffTestTimeout = r.Timeout
	}
	if !f.Changed("concurrency") && r.Concurrency > 0 {
		diffTestConcurrency = r.Concurrency
	}
	if !f.Changed("output") && r.Output != "" {
		diffTestOutput = r.Output
	}
	if !f.Changed("report") && r.Report != "" {
		diffTestReport = r.Report
	}
	if !f.Changed("csv") && r.CSV != "" {
		diffTestCSV = r.CSV
	}
}

func runDiffTest(cmd *cobra.Command, args []string) error {
	fillReplayDefaults(cmd)
	if err := validateOutputFormat(diffTestOutput); err != nil {
		return err
	}
	diffTestRPCs = rpcsFromConfig(diffTestRPCs)
	if len(diffTestRPCs) != 2 {
		return fmt.Errorf("exactly 2 --rpc endpoints are required")
	}
	if len(diffTestOnly) > 0 && (diffTestTraceTransaction || diffTestTraceBlock) {
		return fmt.Errorf("--only cannot be combined with --trace-transaction or --trace-block")
	}
	only, err := parseReplayOnlyTargets(diffTestOnly)
	if err != nil {
		return err
	}

	ds, err := dataset.Load(diffTestDataset)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}

	opts := diff.DefaultOptions()
	for _, f := range diffTestIgnoreFields {
		opts.IgnoreFields[f] = true
	}

	slog.Info("running replay",
		"accounts", len(ds.Accounts),
		"transactions", len(ds.Transactions),
		"blocks", len(ds.Blocks))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := replay.Run(ctx, ds, replay.Config{
		EndpointA:        diffTestRPCs[0],
		EndpointB:        diffTestRPCs[1],
		MaxTxPerAccount:  diffTestMaxTxPerAccount,
		DiffOpts:         opts,
		TraceTransaction: diffTestTraceTransaction,
		TraceBlock:       diffTestTraceBlock,
		Only:             only,
		RPCOptions:       rpcOptions(diffTestTimeout),
		StateFile:        diffTestStateFile,
		Resume:           diffTestResume,
		StateInterval:    diffTestStateInterval,
		DatasetPath:      diffTestDataset,
	}, diffTestConcurrency, os.Stderr)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}

	// Write to stdout.
	printResult(os.Stdout, result, diffTestOutput)

	// Publish replay diff category histogram to the metrics registry so
	// it's exposed at /metrics and, if configured, pushed to the gateway.
	for cat, n := range result.Summary() {
		metrics.ObserveReplayCategory(string(cat), n)
	}

	// Write to report file if requested.
	if diffTestReport != "" {
		f, err := os.Create(diffTestReport)
		if err != nil {
			return fmt.Errorf("create report file: %w", err)
		}
		defer f.Close()
		printResult(f, result, diffTestOutput)
		slog.Info("report written", "path", diffTestReport)
	}

	// Write CSV diff report if requested.
	if diffTestCSV != "" {
		f, err := os.Create(diffTestCSV)
		if err != nil {
			return fmt.Errorf("create CSV report file: %w", err)
		}
		defer f.Close()
		if err := replay.WriteResultCSV(f, result); err != nil {
			return fmt.Errorf("write CSV report: %w", err)
		}
		slog.Info("CSV report written", "path", diffTestCSV)
	}

	t := config.ReplayThresholds{}
	if globalConfig != nil {
		t = globalConfig.Thresholds.Replay
	}
	if cmd.Flags().Changed("max-mismatch-rate") {
		t.MismatchRate = diffTestTMismatchRate
	}
	if cmd.Flags().Changed("max-error-rate") {
		t.ErrorRate = diffTestTErrorRate
	}
	if cmd.Flags().Changed("max-mismatch") {
		t.MaxMismatch = diffTestTMaxMismatch
	}
	configured := thresholds.AnyConfiguredReplay(t)
	breaches := thresholds.EvalReplay(result, t)

	htmlOut, mdOut, junitOut := reportPaths(diffTestReportHTML, diffTestReportMD, diffTestReportJUnit)
	if err := writeFile(htmlOut, func(w *os.File) error { return report.WriteReplayHTML(w, result, breaches, configured) }); err != nil {
		return err
	}
	if err := writeFile(mdOut, func(w *os.File) error { return report.WriteReplayMarkdown(w, result, breaches, configured) }); err != nil {
		return err
	}
	if err := writeFile(junitOut, func(w *os.File) error { return report.WriteReplayJUnit(w, result, breaches) }); err != nil {
		return err
	}

	emitBreaches(breaches, configured)
	return nil
}

// printResult writes the replay result to w in the requested format.
func printResult(w io.Writer, result *replay.Result, format string) {
	if format == "json" {
		replay.PrintResultJSON(w, result)
	} else {
		replay.PrintResult(w, result)
	}
}
