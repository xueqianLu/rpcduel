package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xueqianLu/rpcduel/internal/config"
	"github.com/xueqianLu/rpcduel/internal/thresholds"
)

// applyConfigDefaults populates the global flag variables from a loaded
// config file. CLI flags that the user supplied explicitly take
// precedence (we only fill in values where Changed() is false).
//
// Per-subcommand defaults (bench/duel/diff/replay) are applied separately
// from each command's RunE since the relevant variables live in those
// files; see fillBenchDefaults, fillDuelDefaults, fillDiffDefaults, and
// fillReplayDefaults.
func applyConfigDefaults(cmd *cobra.Command, cfg *config.Config) {
	if cfg == nil {
		return
	}
	d := cfg.Defaults
	pf := cmd.Root().PersistentFlags()
	fill := func(name string, set func()) {
		if !pf.Changed(name) {
			set()
		}
	}
	if d.LogLevel != "" {
		fill("log-level", func() { globalLogLevel = d.LogLevel })
	}
	if d.LogFormat != "" {
		fill("log-format", func() { globalLogFormat = d.LogFormat })
	}
	if d.Retries > 0 {
		fill("retries", func() { globalRetries = d.Retries })
	}
	if d.RetryBackoff > 0 {
		fill("retry-backoff", func() { globalRetryBackoff = d.RetryBackoff })
	}
	if d.UserAgent != "" {
		fill("user-agent", func() { globalUserAgent = d.UserAgent })
	}
	if d.Insecure {
		fill("insecure", func() { globalInsecureTLS = true })
	}
	if d.MetricsAddr != "" {
		fill("metrics-addr", func() { globalMetricsAddr = d.MetricsAddr })
	}
	if d.RPS > 0 {
		fill("rps", func() { globalRPS = d.RPS })
	}
	if d.RPSBurst > 0 {
		fill("rps-burst", func() { globalRPSBurst = d.RPSBurst })
	}
	if len(d.Headers) > 0 && !pf.Changed("header") {
		for k, v := range d.Headers {
			globalHeaders = append(globalHeaders, fmt.Sprintf("%s: %s", k, v))
		}
	}
}

// rpcsFromConfig returns config-supplied endpoint URLs when the user
// didn't pass --rpc. CLI --rpc always takes precedence.
func rpcsFromConfig(current []string) []string {
	if len(current) > 0 {
		return current
	}
	if globalConfig == nil {
		return current
	}
	return globalConfig.EndpointURLs()
}

// reportPaths returns the (html, markdown, junit) output paths combined
// from per-command CLI flags and the config file's reports: section.
// CLI flags win.
func reportPaths(htmlFlag, mdFlag, junitFlag string) (string, string, string) {
	htmlOut := htmlFlag
	mdOut := mdFlag
	junitOut := junitFlag
	if globalConfig != nil {
		if htmlOut == "" {
			htmlOut = globalConfig.Reports.HTML
		}
		if mdOut == "" {
			mdOut = globalConfig.Reports.Markdown
		}
		if junitOut == "" {
			junitOut = globalConfig.Reports.JUnit
		}
	}
	return htmlOut, mdOut, junitOut
}

// emitBreaches prints a PASS/FAIL summary to stderr and, if any breaches
// occurred, calls os.Exit(2). Done in a single helper so every verb
// command's gating behavior is identical.
func emitBreaches(breaches []thresholds.Breach, configured bool) {
	failed := thresholds.Print(os.Stderr, breaches, configured)
	if failed {
		os.Exit(2)
	}
}

// writeFile creates path and invokes write with the open *os.File. A
// path of "" is a no-op (the corresponding report was not requested).
func writeFile(path string, write func(*os.File) error) error {
	if path == "" {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	return write(f)
}
