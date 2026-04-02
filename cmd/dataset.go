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
	Short: "Collect on-chain data by scanning a block range and save a test dataset",
	Long: `Scan blocks from high to low over a given block range via an Ethereum
JSON-RPC endpoint and write a standardised JSON dataset file containing blocks,
transactions, and accounts for use with replay and benchgen.`,
	RunE: runDataset,
}

var (
	datasetRPC             string
	datasetFromBlock       int64
	datasetToBlock         int64
	datasetAccounts        int
	datasetTxs             int
	datasetBlocks          int
	datasetMaxTxPerAccount int
	datasetOut             string
	datasetChain           string
	datasetConcurrency     int
)

func init() {
	datasetCmd.Flags().StringVar(&datasetRPC, "rpc", "", "Ethereum JSON-RPC endpoint URL (required)")
	datasetCmd.Flags().Int64Var(&datasetFromBlock, "from-block", 0, "Start block, inclusive (0 = chain head minus --blocks range)")
	datasetCmd.Flags().Int64Var(&datasetToBlock, "to-block", 0, "End block, inclusive (0 = latest)")
	datasetCmd.Flags().IntVar(&datasetAccounts, "accounts", 1000, "Maximum number of accounts to collect")
	datasetCmd.Flags().IntVar(&datasetTxs, "txs", 1000, "Maximum number of transactions to collect")
	datasetCmd.Flags().IntVar(&datasetBlocks, "blocks", 1000, "Maximum number of blocks to collect")
	datasetCmd.Flags().IntVar(&datasetMaxTxPerAccount, "max-tx-per-account", 100,
		"Maximum transactions to store per account in the dataset (0 = unlimited)")
	datasetCmd.Flags().StringVar(&datasetOut, "out", "dataset.json", "Output file path")
	datasetCmd.Flags().StringVar(&datasetChain, "chain", "ethereum", "Chain name recorded in the dataset")
	datasetCmd.Flags().IntVar(&datasetConcurrency, "concurrency", 4, "Number of goroutines used to fetch blocks from the RPC endpoint")
}

// defaultFromBlockMultiplier is used when --from-block is not specified: we look
// back this many times the requested block count to give enough room to find
// blocks that contain transactions.
const defaultFromBlockMultiplier = 10

func runDataset(cmd *cobra.Command, args []string) error {
	if datasetRPC == "" {
		return fmt.Errorf("--rpc URL is required")
	}

	ctx := context.Background()
	scanner := dataset.NewChainScanner(datasetRPC)

	// Resolve the upper bound of the scan range.
	toBlock := datasetToBlock
	if toBlock == 0 {
		latest, err := scanner.LatestBlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("get latest block: %w", err)
		}
		toBlock = latest
		fmt.Fprintf(os.Stderr, "Latest block: %d\n", toBlock)
	}

	fromBlock := datasetFromBlock
	if fromBlock == 0 {
		// Default: scan backward from toBlock far enough to find data.
		fromBlock = toBlock - int64(datasetBlocks)*defaultFromBlockMultiplier
		if fromBlock < 0 {
			fromBlock = 0
		}
	}

	ds := &dataset.Dataset{
		Meta: dataset.Meta{
			Chain:       datasetChain,
			RPC:         datasetRPC,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Range: dataset.Range{
			From: fromBlock,
			To:   toBlock,
		},
	}

	fmt.Fprintf(os.Stderr, "Scanning blocks %d → %d (high to low) via %s\n", toBlock, fromBlock, datasetRPC)
	fmt.Fprintf(os.Stderr, "  collecting up to %d accounts, %d transactions, %d blocks\n",
		datasetAccounts, datasetTxs, datasetBlocks)

	accounts, txs, blocks, err := scanner.Scan(ctx, fromBlock, toBlock, datasetAccounts, datasetTxs, datasetBlocks, datasetMaxTxPerAccount, datasetConcurrency)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: scan incomplete: %v\n", err)
	}
	ds.Accounts = accounts
	ds.Transactions = txs
	ds.Blocks = blocks

	fmt.Fprintf(os.Stderr, "  collected %d accounts, %d transactions, %d blocks\n",
		len(ds.Accounts), len(ds.Transactions), len(ds.Blocks))

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
