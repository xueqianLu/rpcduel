package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/rpc"
	"github.com/xueqianLu/rpcduel/internal/runner"
)

var duelCmd = &cobra.Command{
	Use:   "duel",
	Short: "Benchmark and diff two RPC endpoints simultaneously",
	Long: `Run concurrent requests against two RPC endpoints, collecting both
performance metrics and response consistency (diff) statistics.`,
	RunE: runDuel,
}

var (
	duelRPCs         []string
	duelMethod       string
	duelParamsStr    string
	duelConcurrency  int
	duelRequests     int
	duelDuration     time.Duration
	duelTimeout      time.Duration
	duelOutput       string
	duelIgnoreFields []string
	duelIgnoreOrder  bool
)

func init() {
	duelCmd.Flags().StringArrayVar(&duelRPCs, "rpc", nil, "RPC endpoint URL (specify exactly 2)")
	duelCmd.Flags().StringVar(&duelMethod, "method", "eth_blockNumber", "JSON-RPC method name")
	duelCmd.Flags().StringVar(&duelParamsStr, "params", "[]", "JSON-encoded params array")
	duelCmd.Flags().IntVar(&duelConcurrency, "concurrency", 10, "Number of concurrent workers")
	duelCmd.Flags().IntVar(&duelRequests, "requests", 100, "Total request pairs (0 = use duration)")
	duelCmd.Flags().DurationVar(&duelDuration, "duration", 0, "Run for this long instead of fixed count (e.g. 30s)")
	duelCmd.Flags().DurationVar(&duelTimeout, "timeout", 30*time.Second, "Per-request timeout")
	duelCmd.Flags().StringVar(&duelOutput, "output", "text", "Output format: text or json")
	duelCmd.Flags().StringArrayVar(&duelIgnoreFields, "ignore-field", nil, "JSON field names to ignore in comparison")
	duelCmd.Flags().BoolVar(&duelIgnoreOrder, "ignore-order", false, "Treat arrays as unordered sets")
}

func runDuel(cmd *cobra.Command, args []string) error {
	if len(duelRPCs) != 2 {
		return fmt.Errorf("exactly 2 --rpc endpoints are required")
	}

	epA := duelRPCs[0]
	epB := duelRPCs[1]

	params, err := rpc.ParseParams(duelParamsStr)
	if err != nil {
		return err
	}

	opts := diff.DefaultOptions()
	for _, f := range duelIgnoreFields {
		opts.IgnoreFields[f] = true
	}
	opts.IgnoreOrder = duelIgnoreOrder

	ctx := context.Background()
	outFmt := report.Format(duelOutput)

	metricsA := bench.NewMetrics(epA)
	metricsB := bench.NewMetrics(epB)
	var allDiffs []diff.Difference
	total := 0

	var pairCh <-chan runner.PairResult

	if duelDuration > 0 {
		pairCh = runner.PairResultFromDuration(ctx, epA, epB, duelMethod, params,
			duelConcurrency, duelDuration, duelTimeout)
	} else {
		if duelRequests <= 0 {
			duelRequests = 100
		}
		pairCh = runner.RunPaired(ctx, epA, epB, duelMethod, params,
			duelConcurrency, duelRequests, duelTimeout)
	}

	for pair := range pairCh {
		total++
		metricsA.Record(pair.Left.Latency, pair.Left.Err != nil)
		metricsB.Record(pair.Right.Latency, pair.Right.Err != nil)

		if pair.Left.Err != nil || pair.Right.Err != nil {
			if pair.Left.Err == nil || pair.Right.Err == nil {
				allDiffs = append(allDiffs, diff.Difference{
					Path:   "$",
					Left:   fmt.Sprintf("%v", pair.Left.Err),
					Right:  fmt.Sprintf("%v", pair.Right.Err),
					Reason: "one endpoint errored",
				})
			}
			continue
		}

		lJSON := runner.ResultToJSON(pair.Left.Response)
		rJSON := runner.ResultToJSON(pair.Right.Response)
		diffs, err := diff.Compare(lJSON, rJSON, opts)
		if err != nil {
			slog.Warn("compare error", "err", err)
			continue
		}
		allDiffs = append(allDiffs, diffs...)
	}

	diffRate := 0.0
	if total > 0 {
		diffRate = float64(len(allDiffs)) / float64(total)
	}

	metricsA.Finish()
	metricsB.Finish()
	sumA := metricsA.Summarize()
	sumB := metricsB.Summarize()

	rep := report.DuelReport{
		Endpoints: []string{epA, epB},
		Method:    duelMethod,
		Total:     total,
		DiffCount: len(allDiffs),
		DiffRate:  diffRate,
		Diffs:     allDiffs,
		Metrics:   []bench.Summary{sumA, sumB},
	}
	report.PrintDuel(os.Stdout, rep, outFmt)
	return nil
}
