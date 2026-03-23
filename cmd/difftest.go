package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/replay"
)

var diffTestCmd = &cobra.Command{
	Use:   "diff-test",
	Short: "Run data-driven consistency tests between two RPC endpoints using a dataset",
	Long: `Load a dataset file (created with rpcduel dataset) and execute RPC calls
for each account, transaction, and block against two endpoints, reporting any
response differences.`,
	RunE: runDiffTest,
}

var (
	diffTestDataset         string
	diffTestRPCs            []string
	diffTestMaxTxPerAccount int
	diffTestOutput          string
	diffTestIgnoreFields    []string
	diffTestTimeout         time.Duration
	diffTestConcurrency     int
	diffTestReport          string
	diffTestCSV             string
)

func init() {
	diffTestCmd.Flags().StringVar(&diffTestDataset, "dataset", "dataset.json", "Path to the dataset JSON file")
	diffTestCmd.Flags().StringArrayVar(&diffTestRPCs, "rpc", nil, "RPC endpoint URL (specify exactly 2)")
	diffTestCmd.Flags().IntVar(&diffTestMaxTxPerAccount, "max-tx-per-account", 100,
		"Maximum transactions to test per account (0 = unlimited)")
	diffTestCmd.Flags().StringVar(&diffTestOutput, "output", "text", "Output format: text or json")
	diffTestCmd.Flags().StringArrayVar(&diffTestIgnoreFields, "ignore-field", nil,
		"JSON field names to ignore in comparison")
	diffTestCmd.Flags().DurationVar(&diffTestTimeout, "timeout", 30*time.Second, "Per-request timeout")
	diffTestCmd.Flags().IntVar(&diffTestConcurrency, "concurrency", 4, "Number of goroutines used to execute RPC calls")
	diffTestCmd.Flags().StringVar(&diffTestReport, "report", "", "Write the report to this file (in addition to stdout)")
	diffTestCmd.Flags().StringVar(&diffTestCSV, "csv", "", "Write a CSV report of all diffs to this file")
}

func runDiffTest(cmd *cobra.Command, args []string) error {
	if len(diffTestRPCs) != 2 {
		return fmt.Errorf("exactly 2 --rpc endpoints are required")
	}

	ds, err := dataset.Load(diffTestDataset)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}

	opts := diff.DefaultOptions()
	for _, f := range diffTestIgnoreFields {
		opts.IgnoreFields[f] = true
	}

	fmt.Fprintf(os.Stderr, "Running diff-test on dataset (accounts=%d txs=%d blocks=%d)...\n",
		len(ds.Accounts), len(ds.Transactions), len(ds.Blocks))

	ctx := context.Background()
	result, err := replay.Run(ctx, ds, diffTestRPCs[0], diffTestRPCs[1], diffTestMaxTxPerAccount, diffTestConcurrency, opts, os.Stderr)
	if err != nil {
		return fmt.Errorf("diff-test: %w", err)
	}

	// Write to stdout.
	printResult(os.Stdout, result, diffTestOutput)

	// Write to report file if requested.
	if diffTestReport != "" {
		f, err := os.Create(diffTestReport)
		if err != nil {
			return fmt.Errorf("create report file: %w", err)
		}
		defer f.Close()
		printResult(f, result, diffTestOutput)
		fmt.Fprintf(os.Stderr, "Report written to %s\n", diffTestReport)
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
		fmt.Fprintf(os.Stderr, "CSV report written to %s\n", diffTestCSV)
	}
	return nil
}

// printResult writes the diff-test result to w in the requested format.
func printResult(w io.Writer, result *replay.Result, format string) {
	if format == "json" {
		replay.PrintResultJSON(w, result)
	} else {
		replay.PrintResult(w, result)
	}
}
