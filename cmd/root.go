package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rpcduel",
	Short: "A CLI tool for comparing and benchmarking Ethereum JSON-RPC endpoints",
	Long: `rpcduel is a high-performance CLI tool for:
	- Calling Ethereum JSON-RPC methods directly (call)
  - Comparing responses from multiple Ethereum JSON-RPC nodes (diff)
  - Benchmarking RPC node performance (bench)
  - Running concurrent diff+benchmark tests (duel)
  - Collecting on-chain test datasets by scanning a block range via RPC (dataset)
			- Replaying dataset-backed consistency checks across nodes (replay)
  - Generating benchmark scenario files from datasets (benchgen)`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(callCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(duelCmd)
	rootCmd.AddCommand(datasetCmd)
	rootCmd.AddCommand(diffTestCmd)
	rootCmd.AddCommand(benchgenCmd)
}
