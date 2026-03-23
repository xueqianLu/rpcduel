package cmd

import (
	"context"
	"fmt"
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
	result, err := replay.Run(ctx, ds, diffTestRPCs[0], diffTestRPCs[1], diffTestMaxTxPerAccount, diffTestConcurrency, opts)
	if err != nil {
		return fmt.Errorf("diff-test: %w", err)
	}

	if diffTestOutput == "json" {
		replay.PrintResultJSON(os.Stdout, result)
	} else {
		replay.PrintResult(os.Stdout, result)
	}
	return nil
}
