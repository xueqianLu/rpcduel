package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

var (
	datasetTarget  string
	datasetFrom    uint64
	datasetToBlock uint64
	datasetOut     string
	datasetWorkers int
	datasetTimeout time.Duration
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Stream blocks into a dataset file",
	Long:  "Scan a block range from one JSON-RPC provider and stream unique blocks, transactions, and addresses into dataset.json.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if datasetFrom > datasetToBlock {
			return fmt.Errorf("--from must be less than or equal to --to-block")
		}

		provider, err := newProvider(datasetTarget, datasetTimeout)
		if err != nil {
			return err
		}

		file, err := os.Create(datasetOut)
		if err != nil {
			return fmt.Errorf("create dataset file %s: %w", datasetOut, err)
		}
		defer file.Close()

		collector := dataset.NewCollector(provider, dataset.WithConcurrency(datasetWorkers))
		summary, err := collector.Collect(context.Background(), datasetFrom, datasetToBlock, file)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", datasetOut)
		fmt.Fprintf(cmd.OutOrStdout(), "  blocks:       %d\n", summary.Blocks)
		fmt.Fprintf(cmd.OutOrStdout(), "  transactions: %d\n", summary.Transactions)
		fmt.Fprintf(cmd.OutOrStdout(), "  addresses:    %d\n", summary.Addresses)
		return nil
	},
}

func init() {
	datasetCmd.Flags().StringVar(&datasetTarget, "to", "", "RPC target alias or URL")
	datasetCmd.Flags().Uint64Var(&datasetFrom, "from", 0, "Start block (inclusive)")
	datasetCmd.Flags().Uint64Var(&datasetToBlock, "to-block", 0, "End block (inclusive)")
	datasetCmd.Flags().StringVar(&datasetOut, "out", "dataset.json", "Output dataset path")
	datasetCmd.Flags().IntVar(&datasetWorkers, "concurrency", 8, "Number of concurrent block fetch workers")
	datasetCmd.Flags().DurationVar(&datasetTimeout, "timeout", 15*time.Second, "Per-request timeout")

	_ = datasetCmd.MarkFlagRequired("to")
	_ = datasetCmd.MarkFlagRequired("to-block")
}
