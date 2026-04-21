package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Collect on-chain data by scanning a block range and save a test dataset",
	Long: `Scan blocks from high to low over a given block range via an Ethereum
JSON-RPC endpoint and write a standardized JSON dataset file containing blocks,
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
	datasetAppend          bool
)

func init() {
	datasetCmd.Flags().StringVar(&datasetRPC, "rpc", "", "Ethereum JSON-RPC endpoint URL (required)")
	datasetCmd.Flags().Int64Var(&datasetFromBlock, "from-block", 0, "Start block, inclusive (0 = chain head minus --blocks range, or last-collected+1 in --append mode)")
	datasetCmd.Flags().Int64Var(&datasetToBlock, "to-block", 0, "End block, inclusive (0 = latest)")
	datasetCmd.Flags().IntVar(&datasetAccounts, "accounts", 1000, "Maximum number of accounts to collect")
	datasetCmd.Flags().IntVar(&datasetTxs, "txs", 1000, "Maximum number of transactions to collect")
	datasetCmd.Flags().IntVar(&datasetBlocks, "blocks", 1000, "Maximum number of blocks to collect")
	datasetCmd.Flags().IntVar(&datasetMaxTxPerAccount, "max-tx-per-account", 100,
		"Maximum transactions to store per account in the dataset (0 = unlimited)")
	datasetCmd.Flags().StringVar(&datasetOut, "out", "dataset.json", "Output file path")
	datasetCmd.Flags().StringVar(&datasetChain, "chain", "ethereum", "Chain name recorded in the dataset")
	datasetCmd.Flags().IntVar(&datasetConcurrency, "concurrency", 4, "Number of goroutines used to fetch blocks from the RPC endpoint")
	datasetCmd.Flags().BoolVar(&datasetAppend, "append", false, "Append to an existing --out dataset: scan only the delta range (last-collected+1 to head by default) and merge results")

	datasetCmd.AddCommand(datasetInspectCmd)
}

var datasetInspectJSON bool

var datasetInspectCmd = &cobra.Command{
	Use:   "inspect <file>",
	Short: "Print summary statistics for a dataset file",
	Long: `Read a dataset JSON file and print metadata, counts, top accounts,
the tx-per-account distribution, and the estimated number of RPC calls each
replay/benchgen category would issue against it.

Use --json for machine-readable output.`,
	Args: cobra.ExactArgs(1),
	RunE: runDatasetInspect,
}

func init() {
	datasetInspectCmd.Flags().BoolVar(&datasetInspectJSON, "json", false, "Emit JSON instead of human-readable text")
}

func runDatasetInspect(_ *cobra.Command, args []string) error {
	path := args[0]
	ds, err := dataset.Load(path)
	if err != nil {
		return err
	}
	stats := dataset.Inspect(path, ds)
	if datasetInspectJSON {
		return dataset.PrintStatsJSON(os.Stdout, stats)
	}
	dataset.PrintStats(os.Stdout, stats)
	return nil
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
	scanner := dataset.NewChainScannerWithOptions(datasetRPC, rpcOptions(30*time.Second))

	// Append mode: load any existing dataset to seed merge + default fromBlock.
	var existing *dataset.Dataset
	if datasetAppend {
		if loaded, err := dataset.Load(datasetOut); err == nil {
			existing = loaded
			slog.Info("append mode: loaded existing dataset",
				"file", datasetOut,
				"accounts", len(existing.Accounts),
				"transactions", len(existing.Transactions),
				"blocks", len(existing.Blocks),
				"prev_range_from", existing.Range.From,
				"prev_range_to", existing.Range.To)
		} else if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
			slog.Info("append mode: no existing dataset, starting fresh", "file", datasetOut)
		} else {
			return fmt.Errorf("append: load existing dataset: %w", err)
		}
	}

	// Resolve the upper bound of the scan range.
	toBlock := datasetToBlock
	if toBlock == 0 {
		latest, err := scanner.LatestBlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("get latest block: %w", err)
		}
		toBlock = latest
		slog.Info("resolved chain head", "latest_block", toBlock)
	}

	fromBlock := datasetFromBlock
	if fromBlock == 0 {
		switch {
		case existing != nil && existing.Range.To > 0:
			fromBlock = existing.Range.To + 1
			if fromBlock > toBlock {
				slog.Info("append mode: nothing new to scan; re-saving existing dataset",
					"prev_to", existing.Range.To, "head", toBlock)
				if err := dataset.Save(datasetOut, existing); err != nil {
					return fmt.Errorf("save dataset: %w", err)
				}
				return nil
			}
		default:
			// Default: scan backward from toBlock far enough to find data.
			fromBlock = toBlock - int64(datasetBlocks)*defaultFromBlockMultiplier
			if fromBlock < 0 {
				fromBlock = 0
			}
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

	slog.Info("scanning blocks",
		"from", fromBlock, "to", toBlock,
		"max_accounts", datasetAccounts, "max_txs", datasetTxs, "max_blocks", datasetBlocks,
		"append", datasetAppend, "rpc", datasetRPC)

	accounts, txs, blocks, err := scanner.Scan(ctx, fromBlock, toBlock, datasetAccounts, datasetTxs, datasetBlocks, datasetMaxTxPerAccount, datasetConcurrency)
	if err != nil {
		slog.Warn("scan incomplete", "err", err)
	}
	ds.Accounts = accounts
	ds.Transactions = txs
	ds.Blocks = blocks

	slog.Info("scan complete",
		"new_accounts", len(ds.Accounts), "new_transactions", len(ds.Transactions), "new_blocks", len(ds.Blocks))

	if existing != nil {
		ds = dataset.Merge(existing, ds, datasetAccounts, datasetTxs, datasetBlocks, datasetMaxTxPerAccount)
		slog.Info("merged with existing dataset",
			"accounts", len(ds.Accounts), "transactions", len(ds.Transactions), "blocks", len(ds.Blocks))
	}

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
