package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xueqianLu/rpcduel/internal/clilog"
	"github.com/xueqianLu/rpcduel/internal/rpc"
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
		return clilog.Setup(globalLogLevel, globalLogFormat)
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

	rootCmd.AddCommand(callCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(duelCmd)
	rootCmd.AddCommand(datasetCmd)
	rootCmd.AddCommand(diffTestCmd)
	rootCmd.AddCommand(benchgenCmd)
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
