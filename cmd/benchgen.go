package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/benchgen"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

var benchgenCmd = &cobra.Command{
	Use:   "benchgen",
	Short: "Generate a benchmark scenario file from a dataset",
	Long: `Load a dataset file (created with rpcduel dataset) and produce a bench.json
file containing request scenarios for use with rpcduel bench --input.

Scenarios generated (basic):
  balance, transaction_count, transaction_by_hash, transaction_receipt, block_by_number

Scenarios generated (complex):
  get_logs, debug_trace_transaction, debug_trace_block, mixed_balance`,
	RunE: runBenchgen,
}

var (
	benchgenDataset string
	benchgenOut     string
)

func init() {
	benchgenCmd.Flags().StringVar(&benchgenDataset, "dataset", "dataset.json", "Path to the dataset JSON file")
	benchgenCmd.Flags().StringVar(&benchgenOut, "out", "bench.json", "Output benchmark scenario file path")
}

func runBenchgen(cmd *cobra.Command, args []string) error {
	ds, err := dataset.Load(benchgenDataset)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}

	bf := benchgen.Generate(ds, nil)

	if err := benchgen.SaveBenchFile(benchgenOut, bf); err != nil {
		return fmt.Errorf("save bench file: %w", err)
	}

	totalReqs := 0
	for _, s := range bf.Scenarios {
		totalReqs += len(s.Requests)
	}

	fmt.Fprintf(os.Stdout, "Bench file saved to %s\n", benchgenOut)
	fmt.Fprintf(os.Stdout, "  scenarios: %d\n", len(bf.Scenarios))
	fmt.Fprintf(os.Stdout, "  total requests: %d\n", totalReqs)
	for _, s := range bf.Scenarios {
		fmt.Fprintf(os.Stdout, "  %-30s  %d requests\n", s.Name, len(s.Requests))
	}
	return nil
}
