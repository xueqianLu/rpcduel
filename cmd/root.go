// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"github.com/xueqianLu/rpcduel/internal/clilog"
	"github.com/xueqianLu/rpcduel/internal/config"
	"github.com/xueqianLu/rpcduel/internal/metrics"
	"github.com/xueqianLu/rpcduel/internal/rpc"
	"github.com/xueqianLu/rpcduel/internal/runner"
)

// buildVersion is set by main via SetBuildInfo and used as the default
// User-Agent on outbound RPC requests when no override is supplied.
var buildVersion = "dev"

// SetBuildInfo is called from main to inject ldflags-provided build metadata.
func SetBuildInfo(version, commit, date string) {
	buildVersion = version
	rootCmd.Version = fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}

// Global flags shared by all subcommands. Wired up in init().
var (
	globalLogLevel     string
	globalLogFormat    string
	globalRetries      int
	globalRetryBackoff time.Duration
	globalHeaders      []string
	globalUserAgent    string
	globalInsecureTLS  bool
	globalMetricsAddr  string
	globalRPS          float64
	globalRPSBurst     int
	globalConfigPath   string
	globalConfig       *config.Config
	globalPushGateway  string
	globalPushJob      string
	globalPushLabels   []string

	// metricsCtx is canceled when the root command exits to gracefully
	// shut down the metrics HTTP server (if one was started).
	metricsCtx    context.Context
	metricsCancel context.CancelFunc
)

var rootCmd = &cobra.Command{
	Use:     "rpcduel",
	Version: "dev",
	Short:   "A CLI tool for comparing and benchmarking Ethereum JSON-RPC endpoints",
	Long: `rpcduel is a high-performance CLI tool for:
  - Calling Ethereum JSON-RPC methods directly (call)
  - Comparing responses from multiple Ethereum JSON-RPC nodes (diff)
  - Benchmarking RPC node performance (bench)
  - Running concurrent diff+benchmark tests (duel)
  - Collecting on-chain test datasets by scanning a block range via RPC (dataset)
  - Replaying dataset-backed consistency checks across nodes (replay)
  - Generating benchmark scenario files from datasets (benchgen)`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if globalConfigPath != "" {
			cfg, err := config.Load(globalConfigPath)
			if err != nil {
				return err
			}
			globalConfig = cfg
			applyConfigDefaults(cmd, cfg)
		}
		if err := clilog.Setup(globalLogLevel, globalLogFormat); err != nil {
			return err
		}
		if globalMetricsAddr != "" {
			metricsCtx, metricsCancel = signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			if err := metrics.StartServer(metricsCtx, globalMetricsAddr); err != nil {
				return fmt.Errorf("start metrics server: %w", err)
			}
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if globalPushGateway != "" {
			labels := parseLabels(globalPushLabels)
			if err := metrics.Push(globalPushGateway, globalPushJob, labels); err != nil {
				fmt.Fprintf(os.Stderr, "push-gateway: %v\n", err)
			}
		}
		if metricsCancel != nil {
			metricsCancel()
		}
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&globalLogLevel, "log-level", "info", "Log level (debug|info|warn|error)")
	pf.StringVar(&globalLogFormat, "log-format", "text", "Log format (text|json)")
	pf.IntVar(&globalRetries, "retries", 0, "Maximum number of RPC retries on transient errors (5xx, 429, network)")
	pf.DurationVar(&globalRetryBackoff, "retry-backoff", 200*time.Millisecond, "Base backoff between RPC retries (exponential)")
	pf.StringArrayVar(&globalHeaders, "header", nil, "Extra HTTP header to send with every RPC request (repeatable, format Key: Value or Key=Value)")
	pf.StringVar(&globalUserAgent, "user-agent", "", "Override the HTTP User-Agent header sent with every RPC request")
	pf.BoolVar(&globalInsecureTLS, "insecure", false, "Skip TLS certificate verification on outbound HTTPS requests (development only)")
	pf.StringVar(&globalMetricsAddr, "metrics-addr", "", "If set (e.g. :9090), expose Prometheus metrics at /metrics on this address")
	pf.Float64Var(&globalRPS, "rps", 0, "Aggregate request rate cap in requests per second (0 = unlimited)")
	pf.IntVar(&globalRPSBurst, "rps-burst", 0, "Token-bucket burst size for --rps (default: 1, or rounded-up rps)")
	pf.StringVarP(&globalConfigPath, "config", "c", "", "Path to rpcduel.yaml config file")
	pf.StringVar(&globalPushGateway, "push-gateway", "", "Prometheus Pushgateway URL (e.g. http://pushgateway:9091). Metrics are pushed at command exit.")
	pf.StringVar(&globalPushJob, "push-job", "rpcduel", "Job label used when pushing to the Pushgateway")
	pf.StringArrayVar(&globalPushLabels, "push-label", nil, "Additional Pushgateway grouping label in key=value form (repeatable)")

	rootCmd.AddCommand(callCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(duelCmd)
	rootCmd.AddCommand(datasetCmd)
	rootCmd.AddCommand(diffTestCmd)
	rootCmd.AddCommand(benchgenCmd)
	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(contractCmd)
}

// runnerContext returns a context wired with the runner-side options derived
// from global flags: rpc.Options (so retries/headers/insecure/UA propagate
// into the worker pool) and an optional rate.Limiter (--rps).
func runnerContext(parent context.Context) context.Context {
	ctx := runner.WithClientOptions(parent, rpcOptions(0))
	if globalRPS > 0 {
		burst := globalRPSBurst
		if burst <= 0 {
			burst = int(globalRPS)
			if burst < 1 {
				burst = 1
			}
		}
		ctx = runner.WithRateLimiter(ctx, rate.NewLimiter(rate.Limit(globalRPS), burst))
	}
	return ctx
}

// rpcOptions returns the rpc.Options derived from global flags, with the
// supplied per-command timeout applied.
func rpcOptions(timeout time.Duration) rpc.Options {
	ua := globalUserAgent
	if ua == "" {
		ua = "rpcduel/" + buildVersion
	}
	return rpc.Options{
		Timeout:            timeout,
		Retries:            globalRetries,
		RetryBackoff:       globalRetryBackoff,
		Headers:            parseHeaders(globalHeaders),
		UserAgent:          ua,
		InsecureSkipVerify: globalInsecureTLS,
	}
}

// newRPCClient is a convenience wrapper that builds an rpc.Client honoring
// global flags.
func newRPCClient(endpoint string, timeout time.Duration) *rpc.Client {
	return rpc.NewClientWithOptions(endpoint, rpcOptions(timeout))
}

// validateOutputFormat returns an error if format is not one of "text" or
// "json". Used by every subcommand that exposes an --output flag so we
// surface bad values early with a helpful message instead of falling
// through to a silent default.
func validateOutputFormat(format string) error {
	switch format {
	case "text", "json":
		return nil
	default:
		return fmt.Errorf("invalid --output %q: must be one of text, json", format)
	}
}

func parseHeaders(in []string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for _, h := range in {
		if h == "" {
			continue
		}
		// Accept both "Key: Value" and "Key=Value".
		var k, v string
		if idx := strings.Index(h, ":"); idx > 0 {
			k = strings.TrimSpace(h[:idx])
			v = strings.TrimSpace(h[idx+1:])
		} else if idx := strings.Index(h, "="); idx > 0 {
			k = strings.TrimSpace(h[:idx])
			v = strings.TrimSpace(h[idx+1:])
		} else {
			continue
		}
		if k != "" {
			out[k] = v
		}
	}
	return out
}

// parseLabels parses "key=value" grouping labels for the Pushgateway.
func parseLabels(in []string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for _, s := range in {
		idx := strings.Index(s, "=")
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(s[:idx])
		v := strings.TrimSpace(s[idx+1:])
		if k != "" {
			out[k] = v
		}
	}
	return out
}
