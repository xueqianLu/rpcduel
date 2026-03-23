package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Collect on-chain data from Blockscout and save a test dataset",
	Long: `Fetch top accounts, transactions, and blocks from a Blockscout REST API
and write a standardised JSON dataset file for use with diff-test and benchgen.`,
	RunE: runDataset,
}

var (
	datasetBlockscout string
	datasetRPC        string
	datasetFromBlock  int64
	datasetToBlock    int64
	datasetAccounts   int
	datasetTxs        int
	datasetBlocks     int
	datasetOut        string
	datasetChain      string
	datasetRateLimit  int
)

func init() {
	datasetCmd.Flags().StringVar(&datasetBlockscout, "blockscout", "", "Blockscout base URL (e.g. https://blockscout.example.com)")
	datasetCmd.Flags().StringVar(&datasetRPC, "rpc", "", "RPC endpoint (reserved for future fallback use)")
	datasetCmd.Flags().Int64Var(&datasetFromBlock, "from-block", 0, "Start block (inclusive, 0 = no lower bound)")
	datasetCmd.Flags().Int64Var(&datasetToBlock, "to-block", 0, "End block (inclusive, 0 = no upper bound)")
	datasetCmd.Flags().IntVar(&datasetAccounts, "accounts", 1000, "Maximum number of accounts to collect")
	datasetCmd.Flags().IntVar(&datasetTxs, "txs", 1000, "Maximum number of transactions to collect")
	datasetCmd.Flags().IntVar(&datasetBlocks, "blocks", 1000, "Maximum number of blocks to collect")
	datasetCmd.Flags().StringVar(&datasetOut, "out", "dataset.json", "Output file path")
	datasetCmd.Flags().StringVar(&datasetChain, "chain", "ethereum", "Chain name recorded in the dataset")
	datasetCmd.Flags().IntVar(&datasetRateLimit, "rate-limit", 5, "Maximum Blockscout API requests per second")
}

func runDataset(cmd *cobra.Command, args []string) error {
	if datasetBlockscout == "" {
		return fmt.Errorf("--blockscout URL is required")
	}

	ctx := context.Background()
	client := dataset.NewBlockscoutClient(datasetBlockscout, datasetRateLimit)

	ds := &dataset.Dataset{
		Meta: dataset.Meta{
			Chain:       datasetChain,
			Blockscout:  datasetBlockscout,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Range: dataset.Range{
			From: datasetFromBlock,
			To:   datasetToBlock,
		},
	}

	// Fetch accounts
	fmt.Fprintf(os.Stderr, "Fetching top %d accounts...\n", datasetAccounts)
	accounts, err := client.FetchAccounts(ctx, datasetAccounts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: accounts fetch incomplete: %v\n", err)
	}
	ds.Accounts = accounts
	fmt.Fprintf(os.Stderr, "  collected %d accounts\n", len(ds.Accounts))

	// Fetch transactions
	fmt.Fprintf(os.Stderr, "Fetching up to %d transactions (blocks %d–%d)...\n",
		datasetTxs, datasetFromBlock, datasetToBlock)
	txs, err := client.FetchTransactions(ctx, datasetFromBlock, datasetToBlock, datasetTxs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: transactions fetch incomplete: %v\n", err)
	}
	ds.Transactions = txs
	fmt.Fprintf(os.Stderr, "  collected %d transactions\n", len(ds.Transactions))

	// Fetch blocks
	fmt.Fprintf(os.Stderr, "Fetching up to %d blocks (range %d–%d)...\n",
		datasetBlocks, datasetFromBlock, datasetToBlock)
	blocks, err := client.FetchBlocks(ctx, datasetFromBlock, datasetToBlock, datasetBlocks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: blocks fetch incomplete: %v\n", err)
	}
	ds.Blocks = blocks
	fmt.Fprintf(os.Stderr, "  collected %d blocks\n", len(ds.Blocks))

	// Persist
	if err := dataset.Save(datasetOut, ds); err != nil {
		return fmt.Errorf("save dataset: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Dataset saved to %s\n", datasetOut)
	fmt.Fprintf(os.Stdout, "  accounts:     %d\n", len(ds.Accounts))
	fmt.Fprintf(os.Stdout, "  transactions: %d\n", len(ds.Transactions))
	fmt.Fprintf(os.Stdout, "  blocks:       %d\n", len(ds.Blocks))
	return nil
}
