package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/report"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

var callCmd = &cobra.Command{
	Use:   "call [method] [param...]",
	Short: "Call a JSON-RPC method directly",
	Long: `Send a single JSON-RPC request to one endpoint and print the result.
This is a convenient alternative to hand-written curl commands when exploring
or debugging Ethereum JSON-RPC methods.`,
	RunE: runCall,
}

var (
	callRPC        string
	callMethod     string
	callParamsStr  string
	callParamsFile string
	callTimeout    time.Duration
	callOutput     string
)

func init() {
	callCmd.Flags().StringVar(&callRPC, "rpc", "", "RPC endpoint URL")
	callCmd.Flags().StringVar(&callMethod, "method", "eth_blockNumber", "JSON-RPC method name")
	callCmd.Flags().StringVar(&callParamsStr, "params", "[]", "JSON-encoded params array")
	callCmd.Flags().StringVar(&callParamsFile, "params-file", "", "Path to a JSON file containing the params array; overrides --params")
	callCmd.Flags().DurationVar(&callTimeout, "timeout", 30*time.Second, "Per-request timeout")
	callCmd.Flags().StringVar(&callOutput, "output", "text", "Output format: text or json")
	_ = callCmd.MarkFlagRequired("rpc")
}

func runCall(cmd *cobra.Command, args []string) error {
	if callRPC == "" {
		return fmt.Errorf("--rpc is required")
	}

	method, err := resolveCallMethod(cmd, args)
	if err != nil {
		return err
	}

	params, err := loadCallParams(cmd, args)
	if err != nil {
		return err
	}

	client := newRPCClient(callRPC, callTimeout)
	resp, latency, err := client.Call(context.Background(), method, params)

	rep := report.CallReport{
		Endpoint:  callRPC,
		Method:    method,
		Params:    params,
		Success:   err == nil,
		LatencyMS: float64(latency) / float64(time.Millisecond),
	}

	if resp != nil && len(resp.Result) > 0 {
		rep.Result = append(json.RawMessage(nil), resp.Result...)
	}

	if err != nil {
		callErr := &report.CallError{
			Type:    "transport",
			Message: err.Error(),
		}
		if resp != nil && resp.Error != nil {
			callErr.Type = "rpc"
			code := resp.Error.Code
			callErr.Code = &code
			callErr.Message = resp.Error.Message
		}
		rep.Error = callErr
	}

	report.PrintCall(os.Stdout, rep, report.Format(callOutput))
	if err != nil {
		return err
	}
	return nil
}

func resolveCallMethod(cmd *cobra.Command, args []string) (string, error) {
	if len(args) == 0 {
		return callMethod, nil
	}
	if cmd.Flags().Changed("method") {
		return "", fmt.Errorf("cannot use positional method and --method together")
	}
	return args[0], nil
}

func loadCallParams(cmd *cobra.Command, args []string) ([]interface{}, error) {
	if cmd.Flags().Changed("params-file") {
		if len(args) > 1 {
			return nil, fmt.Errorf("cannot use positional params and --params-file together")
		}
		if cmd.Flags().Changed("params") {
			return nil, fmt.Errorf("cannot use --params and --params-file together")
		}
		return loadCallParamsFromFile()
	}

	if cmd.Flags().Changed("params") {
		if len(args) > 1 {
			return nil, fmt.Errorf("cannot use positional params and --params together")
		}
		return rpc.ParseParams(callParamsStr)
	}

	if len(args) > 1 {
		return rpc.ParsePositionalParams(args[1:])
	}

	if callParamsFile != "" {
		return loadCallParamsFromFile()
	}

	return rpc.ParseParams(callParamsStr)
}

func loadCallParamsFromFile() ([]interface{}, error) {
	data, err := os.ReadFile(callParamsFile)
	if err != nil {
		return nil, fmt.Errorf("read params file: %w", err)
	}

	var params []interface{}
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, fmt.Errorf("parse params file: %w", err)
	}
	return params, nil
}
