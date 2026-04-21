// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/xueqianLu/rpcduel/internal/doctor"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

var (
	doctorRPCs    []string
	doctorTimeout time.Duration
	doctorOutput  string
	doctorExtra   []string
	doctorFailOn  string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Probe JSON-RPC endpoints for connectivity, identity, sync state, and method capability",
	Long: `doctor runs a battery of lightweight JSON-RPC probes against each endpoint:

  - web3_clientVersion  (identifies the node software)
  - eth_chainId         (detects chain-id mismatches between endpoints)
  - eth_blockNumber     (reports chain tip)
  - eth_syncing         (flags nodes that are still catching up)
  - net_peerCount
  - eth_gasPrice

Additional methods can be probed with --probe to verify capability
matrices (e.g. "debug_traceBlockByNumber"). With --fail-on any or
--fail-on unreachable, doctor exits 2 on failure so it can gate CI
steps.`,
	RunE:         runDoctor,
	SilenceUsage: true,
}

func init() {
	doctorCmd.Flags().StringArrayVar(&doctorRPCs, "rpc", nil, "RPC endpoint URL (can be specified multiple times)")
	doctorCmd.Flags().DurationVar(&doctorTimeout, "timeout", 5*time.Second, "Per-request timeout")
	doctorCmd.Flags().StringVar(&doctorOutput, "output", "text", "Output format: text or json")
	doctorCmd.Flags().StringArrayVar(&doctorExtra, "probe", nil, "Additional JSON-RPC method name to probe for capability (repeatable)")
	doctorCmd.Flags().StringVar(&doctorFailOn, "fail-on", "unreachable", "Exit code 2 if: 'unreachable' (default), 'any' (any probe fails), or 'never'")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	if err := validateOutputFormat(doctorOutput); err != nil {
		return err
	}
	switch doctorFailOn {
	case "never", "unreachable", "any":
	default:
		return fmt.Errorf("invalid --fail-on %q: must be one of never, unreachable, any", doctorFailOn)
	}
	doctorRPCs = rpcsFromConfig(doctorRPCs)
	if len(doctorRPCs) == 0 {
		return doctor.ErrNoEndpoints
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = rootCmd.Context()
	}

	mk := func(u string) *rpc.Client { return newRPCClient(u, doctorTimeout) }
	rep := doctor.Run(ctx, mk, doctorRPCs, doctor.Options{
		Timeout:      doctorTimeout,
		ExtraMethods: doctorExtra,
	})

	switch doctorOutput {
	case "json":
		if err := doctor.PrintJSON(os.Stdout, rep); err != nil {
			return err
		}
	default:
		doctor.PrintText(os.Stdout, rep)
	}

	anyUnreach := false
	anyFail := false
	for _, ep := range rep.Endpoints {
		if !ep.Reachable {
			anyUnreach = true
		}
		if ep.FailedProbes > 0 {
			anyFail = true
		}
	}
	switch doctorFailOn {
	case "unreachable":
		if anyUnreach {
			os.Exit(2)
		}
	case "any":
		if anyUnreach || anyFail {
			os.Exit(2)
		}
	}
	return nil
}
