package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	diffpkg "github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

var (
	diffTargets     []string
	diffFromBlock   uint64
	diffToBlock     uint64
	diffIgnore      []string
	diffTimeout     time.Duration
	diffShowDetails int
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Audit semantic differences across RPC endpoints",
	Long:  "Compare block, transaction, receipt, and account-state responses across multiple endpoints over a block range.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if diffFromBlock > diffToBlock {
			return fmt.Errorf("--from must be less than or equal to --to-block")
		}

		targets, err := resolveTargets(diffTargets)
		if err != nil {
			return err
		}
		if len(targets) < 2 {
			return fmt.Errorf("at least two --to values are required")
		}

		endpoints := make([]diffpkg.Endpoint, 0, len(targets))
		for _, target := range targets {
			endpoints = append(endpoints, diffpkg.Endpoint{
				Target:   target,
				Provider: rpc.NewProvider(target, diffTimeout),
			})
		}

		options := diffpkg.DefaultOptions()
		options.IgnoreFields = diffpkg.NewIgnoreSet(diffIgnore)

		auditor := diffpkg.NewAuditor(endpoints[0], endpoints[1:], options)
		report, err := auditor.AuditBlockRange(context.Background(), diffFromBlock, diffToBlock)
		if err != nil {
			return err
		}

		printDiffReport(cmd, report, diffShowDetails)
		return nil
	},
}

func init() {
	diffCmd.Flags().StringArrayVar(&diffTargets, "to", nil, "RPC target alias or URL (first target is the baseline)")
	diffCmd.Flags().Uint64Var(&diffFromBlock, "from", 0, "Start block (inclusive)")
	diffCmd.Flags().Uint64Var(&diffToBlock, "to-block", 0, "End block (inclusive)")
	diffCmd.Flags().StringArrayVar(&diffIgnore, "ignore", nil, "Field name or path to ignore during semantic diffing")
	diffCmd.Flags().DurationVar(&diffTimeout, "timeout", 15*time.Second, "Per-request timeout")
	diffCmd.Flags().IntVar(&diffShowDetails, "show", 20, "Maximum findings to print")

	_ = diffCmd.MarkFlagRequired("to")
	_ = diffCmd.MarkFlagRequired("to-block")
}

func printDiffReport(cmd *cobra.Command, report *diffpkg.Report, limit int) {
	fmt.Fprintf(cmd.OutOrStdout(), "baseline: %s\n", report.Baseline)
	fmt.Fprintf(cmd.OutOrStdout(), "targets:  %v\n", report.Targets)
	fmt.Fprintf(cmd.OutOrStdout(), "range:    %d-%d\n", report.From, report.To)
	fmt.Fprintf(cmd.OutOrStdout(), "checks:   %d\n", report.Checks)
	fmt.Fprintf(cmd.OutOrStdout(), "findings: %d\n", len(report.Findings))

	if len(report.Findings) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no semantic differences detected")
		return
	}

	if limit <= 0 || limit > len(report.Findings) {
		limit = len(report.Findings)
	}

	for _, finding := range report.Findings[:limit] {
		fmt.Fprintf(cmd.OutOrStdout(), "\n[%s] %s %v\n", finding.Endpoint, finding.Method, finding.Params)
		if finding.Error != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  error: %s\n", finding.Error)
			continue
		}
		for _, difference := range finding.Differences {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", difference)
		}
	}
}
