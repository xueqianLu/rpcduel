package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

var (
	callTarget  string
	callTimeout time.Duration
)

var quantityFieldNames = map[string]struct{}{
	"balance":              {},
	"blockNumber":          {},
	"chainId":              {},
	"cumulativeGasUsed":    {},
	"difficulty":           {},
	"effectiveGasPrice":    {},
	"excessBlobGas":        {},
	"gas":                  {},
	"gasPrice":             {},
	"gasUsed":              {},
	"maxFeePerGas":         {},
	"maxPriorityFeePerGas": {},
	"nonce":                {},
	"number":               {},
	"size":                 {},
	"timestamp":            {},
	"transactionIndex":     {},
	"value":                {},
}

var callCmd = &cobra.Command{
	Use:   "call METHOD [params...]",
	Short: "Call a JSON-RPC method with human-friendly output",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := args[0]
		params, err := parseCLIParams(args[1:])
		if err != nil {
			return err
		}
		params = coerceCallParams(method, params)

		provider, err := newProvider(callTarget, callTimeout)
		if err != nil {
			return err
		}

		response, _, err := provider.Call(context.Background(), method, params...)
		if err != nil {
			return err
		}

		formatted, err := prettyRPCResult(response.Result)
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), formatted)
		return nil
	},
}

func init() {
	callCmd.Flags().StringVar(&callTarget, "to", "", "RPC target alias or URL")
	callCmd.Flags().DurationVar(&callTimeout, "timeout", 15*time.Second, "Per-request timeout")

	_ = callCmd.MarkFlagRequired("to")
}

func prettyRPCResult(raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw), nil
	}

	annotated := annotateValue("", value)
	formatted, err := json.MarshalIndent(annotated, "", "  ")
	if err != nil {
		return "", fmt.Errorf("format RPC result: %w", err)
	}
	return string(formatted), nil
}

func annotateValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = annotateValue(childKey, childValue)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, childValue := range typed {
			out[index] = annotateValue(key, childValue)
		}
		return out
	case string:
		if decimal, ok := annotateQuantity(key, typed); ok {
			return fmt.Sprintf("%s (%s)", typed, decimal)
		}
		return typed
	default:
		return value
	}
}

func annotateQuantity(key, value string) (string, bool) {
	if !strings.HasPrefix(value, "0x") && !strings.HasPrefix(value, "0X") {
		return "", false
	}

	if !shouldAnnotateQuantity(key, value) {
		return "", false
	}

	number, err := rpc.ParseQuantityBig(value)
	if err != nil {
		return "", false
	}
	return number.String(), true
}

func shouldAnnotateQuantity(key, value string) bool {
	if _, ok := quantityFieldNames[key]; ok {
		return true
	}

	trimmed := strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	switch len(trimmed) {
	case 40, 64:
		return false
	default:
		return true
	}
}

func coerceCallParams(method string, params []any) []any {
	out := append([]any(nil), params...)
	for _, position := range blockTagParamPositions[method] {
		if position >= len(out) {
			continue
		}

		text, ok := out[position].(string)
		if !ok {
			continue
		}
		if blockTag, changed := coerceBlockTag(text); changed {
			out[position] = blockTag
		}
	}
	return out
}

var blockTagParamPositions = map[string][]int{
	"debug_traceBlockByNumber":             {0},
	"eth_call":                             {1},
	"eth_estimateGas":                      {1},
	"eth_getBalance":                       {1},
	"eth_getBlockByNumber":                 {0},
	"eth_getBlockReceipts":                 {0},
	"eth_getBlockTransactionCountByNumber": {0},
	"eth_getCode":                          {1},
	"eth_getProof":                         {2},
	"eth_getStorageAt":                     {2},
	"eth_getTransactionCount":              {1},
	"eth_getUncleCountByBlockNumber":       {0},
}

func coerceBlockTag(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value, false
	}
	if isNamedBlockTag(trimmed) || strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
		return trimmed, false
	}

	number, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return value, false
	}
	return rpc.HexBlockNumber(number), true
}

func isNamedBlockTag(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "earliest", "finalized", "latest", "pending", "safe":
		return true
	default:
		return false
	}
}
