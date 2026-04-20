package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// BatchRequest holds a JSON-RPC request from an input file.
type BatchRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare responses from multiple RPC endpoints",
	Long: `Send the same JSON-RPC request to multiple endpoints and compare the responses.
Supports single requests via flags or batch requests from a JSON file.`,
	RunE: runDiff,
}

var (
	diffRPCs        []string
	diffMethod      string
	diffParamsStr   string
	diffInputFile   string
	diffRepeat      int
	diffOutput      string
	diffIgnoreFields []string
	diffIgnoreOrder  bool
	diffTimeout     time.Duration
)

func init() {
	diffCmd.Flags().StringArrayVar(&diffRPCs, "rpc", nil, "RPC endpoint URL (can be specified multiple times, minimum 2)")
	diffCmd.Flags().StringVar(&diffMethod, "method", "eth_blockNumber", "JSON-RPC method name")
	diffCmd.Flags().StringVar(&diffParamsStr, "params", "[]", "JSON-encoded params array")
	diffCmd.Flags().StringVar(&diffInputFile, "input", "", "JSON file with batch requests [{method, params}]")
	diffCmd.Flags().IntVar(&diffRepeat, "repeat", 1, "Number of times to repeat the request")
	diffCmd.Flags().StringVar(&diffOutput, "output", "text", "Output format: text or json")
	diffCmd.Flags().StringArrayVar(&diffIgnoreFields, "ignore-field", nil, "JSON field names to ignore in comparison")
	diffCmd.Flags().BoolVar(&diffIgnoreOrder, "ignore-order", false, "Treat arrays as unordered sets")
	diffCmd.Flags().DurationVar(&diffTimeout, "timeout", 30*time.Second, "Request timeout")
}

func runDiff(cmd *cobra.Command, args []string) error {
	if len(diffRPCs) < 2 {
		return fmt.Errorf("at least 2 --rpc endpoints are required")
	}

	// Build diff options
	opts := diff.DefaultOptions()
	for _, f := range diffIgnoreFields {
		opts.IgnoreFields[f] = true
	}
	opts.IgnoreOrder = diffIgnoreOrder

	// Load requests
	requests, err := loadRequests()
	if err != nil {
		return err
	}

	outFmt := report.Format(diffOutput)
	var allDiffs []diff.Difference
	total := 0

	// Compare first two endpoints
	epA := diffRPCs[0]
	epB := diffRPCs[1]

	ctx := context.Background()

	for _, req := range requests {
		for i := 0; i < diffRepeat; i++ {
			total++
			cA := newRPCClient(epA, diffTimeout)
			cB := newRPCClient(epB, diffTimeout)

			respA, _, errA := cA.Call(ctx, req.Method, req.Params)
			respB, _, errB := cB.Call(ctx, req.Method, req.Params)

			if errA != nil && errB != nil {
				// Both errored - consider equal (both failed)
				continue
			}
			if errA != nil || errB != nil {
				allDiffs = append(allDiffs, diff.Difference{
					Path:   "$",
					Left:   fmt.Sprintf("%v", errA),
					Right:  fmt.Sprintf("%v", errB),
					Reason: "one endpoint errored",
				})
				continue
			}

			diffs, err := diff.Compare(respA.Result, respB.Result, opts)
			if err != nil {
				slog.Warn("compare error", "err", err)
				continue
			}
			allDiffs = append(allDiffs, diffs...)
		}
	}

	rep := report.DiffReport{
		Endpoints: []string{epA, epB},
		Method:    requests[0].Method,
		Total:     total,
		DiffCount: len(allDiffs),
		Diffs:     allDiffs,
	}
	report.PrintDiff(os.Stdout, rep, outFmt)
	return nil
}

func loadRequests() ([]BatchRequest, error) {
	if diffInputFile != "" {
		data, err := os.ReadFile(diffInputFile)
		if err != nil {
			return nil, fmt.Errorf("read input file: %w", err)
		}
		var reqs []BatchRequest
		if err := json.Unmarshal(data, &reqs); err != nil {
			return nil, fmt.Errorf("parse input file: %w", err)
		}
		return reqs, nil
	}

	params, err := rpc.ParseParams(diffParamsStr)
	if err != nil {
		return nil, err
	}
	return []BatchRequest{{Method: diffMethod, Params: params}}, nil
}
