package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/benchgen"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Capture real on-chain traffic from an endpoint into a bench scenario file",
	Long: `Scan a block range from an Ethereum JSON-RPC endpoint and emit a bench
scenario file (compatible with rpcduel bench --input and rpcduel duel --input).

This is the one-step equivalent of:
  rpcduel dataset --rpc URL --out tmp.json
  rpcduel benchgen --dataset tmp.json --out bench.json

with extra knobs for filtering by RPC method and sampling.`,
	RunE: runRecord,
}

var (
	recordRPC         string
	recordOut         string
	recordFromBlock   int64
	recordToBlock     int64
	recordMaxBlocks   int
	recordMaxTxs      int
	recordMaxAccounts int
	recordMaxTxPerAcc int
	recordMethods     []string
	recordSample      float64
	recordTraceTx     bool
	recordTraceBlock  bool
	recordConcurrency int
	recordSeed        int64
	recordChain       string
)

func init() {
	f := recordCmd.Flags()
	f.StringVar(&recordRPC, "rpc", "", "Ethereum JSON-RPC endpoint URL (required)")
	f.StringVar(&recordOut, "out", "bench.json", "Output bench scenario file path")
	f.Int64Var(&recordFromBlock, "from-block", 0, "Start block, inclusive (0 = latest minus --max-blocks * 10)")
	f.Int64Var(&recordToBlock, "to-block", 0, "End block, inclusive (0 = latest)")
	f.IntVar(&recordMaxBlocks, "max-blocks", 200, "Maximum number of blocks to ingest")
	f.IntVar(&recordMaxTxs, "max-txs", 2000, "Maximum number of transactions to ingest")
	f.IntVar(&recordMaxAccounts, "max-accounts", 1000, "Maximum number of accounts to ingest")
	f.IntVar(&recordMaxTxPerAcc, "max-tx-per-account", 50, "Maximum transactions stored per account (0 = unlimited)")
	f.StringSliceVar(&recordMethods, "methods", nil,
		"Only emit requests for these RPC methods (comma-separated, case-insensitive). Empty = keep all.")
	f.Float64Var(&recordSample, "sample", 1.0, "Per-scenario sampling fraction in (0,1]. 1.0 = keep everything.")
	f.BoolVar(&recordTraceTx, "trace-transaction", false, "Include debug_traceTransaction scenarios")
	f.BoolVar(&recordTraceBlock, "trace-block", false, "Include debug_traceBlockByNumber scenarios")
	f.IntVar(&recordConcurrency, "concurrency", 4, "Goroutines used to fetch blocks")
	f.Int64Var(&recordSeed, "seed", 42, "Seed for deterministic sampling")
	f.StringVar(&recordChain, "chain", "ethereum", "Chain name recorded in dataset metadata")
}

func runRecord(_ *cobra.Command, _ []string) error {
	if recordRPC == "" {
		return fmt.Errorf("--rpc URL is required")
	}
	if recordSample <= 0 || recordSample > 1 {
		return fmt.Errorf("--sample must be in (0, 1], got %v", recordSample)
	}
	if recordMaxBlocks <= 0 {
		return fmt.Errorf("--max-blocks must be > 0")
	}

	ctx := context.Background()
	scanner := dataset.NewChainScannerWithOptions(recordRPC, rpcOptions(30*time.Second))

	toBlock := recordToBlock
	if toBlock == 0 {
		latest, err := scanner.LatestBlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("get latest block: %w", err)
		}
		toBlock = latest
	}
	fromBlock := recordFromBlock
	if fromBlock == 0 {
		fromBlock = toBlock - int64(recordMaxBlocks)*10
		if fromBlock < 0 {
			fromBlock = 0
		}
	}
	if fromBlock > toBlock {
		return fmt.Errorf("--from-block (%d) must be <= --to-block (%d)", fromBlock, toBlock)
	}

	slog.Info("recording traffic",
		"rpc", recordRPC, "from", fromBlock, "to", toBlock,
		"max_blocks", recordMaxBlocks, "max_txs", recordMaxTxs,
		"sample", recordSample, "methods", recordMethods)

	accounts, txs, blocks, err := scanner.Scan(ctx, fromBlock, toBlock,
		recordMaxAccounts, recordMaxTxs, recordMaxBlocks, recordMaxTxPerAcc, recordConcurrency)
	if err != nil {
		slog.Warn("scan incomplete", "err", err)
	}
	ds := &dataset.Dataset{
		Meta: dataset.Meta{
			Chain:       recordChain,
			RPC:         recordRPC,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Range:        dataset.Range{From: fromBlock, To: toBlock},
		Accounts:     accounts,
		Transactions: txs,
		Blocks:       blocks,
	}

	rng := rand.New(rand.NewSource(recordSeed))
	bf := benchgen.GenerateWithOptions(ds, rng, benchgen.Options{
		TraceTransaction: recordTraceTx,
		TraceBlock:       recordTraceBlock,
	})
	bf = benchgen.FilterMethods(bf, recordMethods)
	bf = benchgen.Sample(bf, recordSample, rng)

	if len(bf.Scenarios) == 0 {
		return fmt.Errorf("no requests after scan/filter/sample (methods=%s, sample=%v)",
			strings.Join(recordMethods, ","), recordSample)
	}
	if err := benchgen.SaveBenchFile(recordOut, bf); err != nil {
		return fmt.Errorf("write bench file: %w", err)
	}

	totalReqs := 0
	for _, s := range bf.Scenarios {
		totalReqs += len(s.Requests)
	}
	fmt.Fprintf(os.Stdout, "Recorded %d scenarios (%d requests) → %s\n",
		len(bf.Scenarios), totalReqs, recordOut)
	for _, s := range bf.Scenarios {
		fmt.Fprintf(os.Stdout, "  %-24s %5d requests  (weight %.2f)\n", s.Name, len(s.Requests), s.Weight)
	}
	return nil
}
