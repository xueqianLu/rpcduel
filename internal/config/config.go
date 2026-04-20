// Package config loads rpcduel.yaml configuration files.
//
// The config file lets users describe endpoints, command defaults,
// SLO thresholds, and report destinations once and reuse them across
// runs (and CI pipelines).
//
// CLI flags always override values from the config file. Values that
// are not specified on either side fall back to per-command defaults.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// Config is the top-level rpcduel.yaml schema.
type Config struct {
	Version    int               `yaml:"version,omitempty"`
	Endpoints  []Endpoint        `yaml:"endpoints,omitempty"`
	Defaults   Defaults          `yaml:"defaults,omitempty"`
	Bench      BenchSection      `yaml:"bench,omitempty"`
	Duel       DuelSection       `yaml:"duel,omitempty"`
	Diff       DiffSection       `yaml:"diff,omitempty"`
	Replay     ReplaySection     `yaml:"replay,omitempty"`
	Thresholds ThresholdsSection `yaml:"thresholds,omitempty"`
	Reports    ReportsSection    `yaml:"reports,omitempty"`
}

// Endpoint describes a single RPC endpoint reference.
type Endpoint struct {
	Name    string            `yaml:"name,omitempty"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

// Defaults are the global runtime defaults applied to every command.
type Defaults struct {
	Timeout      time.Duration     `yaml:"timeout,omitempty"`
	Retries      int               `yaml:"retries,omitempty"`
	RetryBackoff time.Duration     `yaml:"retry_backoff,omitempty"`
	Insecure     bool              `yaml:"insecure,omitempty"`
	UserAgent    string            `yaml:"user_agent,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	RPS          float64           `yaml:"rps,omitempty"`
	RPSBurst     int               `yaml:"rps_burst,omitempty"`
	LogLevel     string            `yaml:"log_level,omitempty"`
	LogFormat    string            `yaml:"log_format,omitempty"`
	MetricsAddr  string            `yaml:"metrics_addr,omitempty"`
}

// BenchSection holds defaults for `rpcduel bench`.
type BenchSection struct {
	Method      string        `yaml:"method,omitempty"`
	Params      string        `yaml:"params,omitempty"`
	Input       string        `yaml:"input,omitempty"`
	Concurrency int           `yaml:"concurrency,omitempty"`
	Requests    int           `yaml:"requests,omitempty"`
	Duration    time.Duration `yaml:"duration,omitempty"`
	Timeout     time.Duration `yaml:"timeout,omitempty"`
	Warmup      time.Duration `yaml:"warmup,omitempty"`
	Output      string        `yaml:"output,omitempty"`
	HDROut      string        `yaml:"hdr_out,omitempty"`
}

// DuelSection holds defaults for `rpcduel duel`.
type DuelSection struct {
	Method       string        `yaml:"method,omitempty"`
	Params       string        `yaml:"params,omitempty"`
	Concurrency  int           `yaml:"concurrency,omitempty"`
	Requests     int           `yaml:"requests,omitempty"`
	Duration     time.Duration `yaml:"duration,omitempty"`
	Timeout      time.Duration `yaml:"timeout,omitempty"`
	Warmup       time.Duration `yaml:"warmup,omitempty"`
	Output       string        `yaml:"output,omitempty"`
	IgnoreFields []string      `yaml:"ignore_fields,omitempty"`
	IgnoreOrder  bool          `yaml:"ignore_order,omitempty"`
}

// DiffSection holds defaults for `rpcduel diff`.
type DiffSection struct {
	Method       string        `yaml:"method,omitempty"`
	Params       string        `yaml:"params,omitempty"`
	Input        string        `yaml:"input,omitempty"`
	Repeat       int           `yaml:"repeat,omitempty"`
	Timeout      time.Duration `yaml:"timeout,omitempty"`
	Output       string        `yaml:"output,omitempty"`
	IgnoreFields []string      `yaml:"ignore_fields,omitempty"`
	IgnoreOrder  bool          `yaml:"ignore_order,omitempty"`
}

// ReplaySection holds defaults for `rpcduel replay`.
type ReplaySection struct {
	Dataset          string        `yaml:"dataset,omitempty"`
	MaxTxPerAccount  int           `yaml:"max_tx_per_account,omitempty"`
	TraceTransaction bool          `yaml:"trace_transaction,omitempty"`
	TraceBlock       bool          `yaml:"trace_block,omitempty"`
	Only             []string      `yaml:"only,omitempty"`
	IgnoreFields     []string      `yaml:"ignore_fields,omitempty"`
	Timeout          time.Duration `yaml:"timeout,omitempty"`
	Concurrency      int           `yaml:"concurrency,omitempty"`
	Output           string        `yaml:"output,omitempty"`
	Report           string        `yaml:"report,omitempty"`
	CSV              string        `yaml:"csv,omitempty"`
}

// ThresholdsSection groups per-command SLO thresholds.
type ThresholdsSection struct {
	Bench  BenchThresholds  `yaml:"bench,omitempty"`
	Duel   DuelThresholds   `yaml:"duel,omitempty"`
	Diff   DiffThresholds   `yaml:"diff,omitempty"`
	Replay ReplayThresholds `yaml:"replay,omitempty"`
}

// BenchThresholds enforces per-endpoint SLOs on a bench run.
//
// A zero value disables the corresponding check. Latency thresholds
// are expressed in milliseconds.
type BenchThresholds struct {
	P50Ms     float64 `yaml:"p50_ms,omitempty"`
	P95Ms     float64 `yaml:"p95_ms,omitempty"`
	P99Ms     float64 `yaml:"p99_ms,omitempty"`
	P999Ms    float64 `yaml:"p999_ms,omitempty"`
	AvgMs     float64 `yaml:"avg_ms,omitempty"`
	ErrorRate float64 `yaml:"error_rate,omitempty"`
	MinQPS    float64 `yaml:"min_qps,omitempty"`
}

// DuelThresholds enforces SLOs on a duel run.
type DuelThresholds struct {
	P95Ms     float64 `yaml:"p95_ms,omitempty"`
	P99Ms     float64 `yaml:"p99_ms,omitempty"`
	ErrorRate float64 `yaml:"error_rate,omitempty"`
	DiffRate  float64 `yaml:"diff_rate,omitempty"`
}

// DiffThresholds enforces SLOs on a diff run.
type DiffThresholds struct {
	DiffRate float64 `yaml:"diff_rate,omitempty"`
	MaxDiffs int     `yaml:"max_diffs,omitempty"`
}

// ReplayThresholds enforces SLOs on a replay run.
type ReplayThresholds struct {
	MismatchRate float64 `yaml:"mismatch_rate,omitempty"`
	ErrorRate    float64 `yaml:"error_rate,omitempty"`
	MaxMismatch  int     `yaml:"max_mismatch,omitempty"`
}

// ReportsSection lists output paths for the report exporters.
type ReportsSection struct {
	HTML     string `yaml:"html,omitempty"`
	Markdown string `yaml:"markdown,omitempty"`
	JUnit    string `yaml:"junit,omitempty"`
}

// Load reads, expands, and parses a YAML configuration file.
//
// Environment variable references of the form ${VAR} or ${VAR:-default}
// are expanded before YAML parsing, allowing users to keep secrets out
// of the file itself. A literal "$$" is replaced with a single "$".
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	expanded := ExpandEnv(string(raw))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Version != 0 && cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config version %d (want 1)", cfg.Version)
	}
	return &cfg, nil
}

var envRefRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// ExpandEnv replaces ${VAR} and ${VAR:-default} sequences in s with their
// environment values. A literal "$$" yields a single "$".
func ExpandEnv(s string) string {
	const sentinel = "\x00DLR\x00"
	s = strings.ReplaceAll(s, "$$", sentinel)
	s = envRefRe.ReplaceAllStringFunc(s, func(match string) string {
		m := envRefRe.FindStringSubmatch(match)
		if v, ok := os.LookupEnv(m[1]); ok {
			return v
		}
		return m[2]
	})
	return strings.ReplaceAll(s, sentinel, "$")
}

// EndpointURLs returns the URLs in declaration order.
func (c *Config) EndpointURLs() []string {
	if c == nil {
		return nil
	}
	urls := make([]string, 0, len(c.Endpoints))
	for _, e := range c.Endpoints {
		urls = append(urls, e.URL)
	}
	return urls
}

// HeadersFor returns merged headers (defaults + per-endpoint) for the
// given URL. Per-endpoint headers win on conflict. Returns nil when
// nothing is configured.
func (c *Config) HeadersFor(url string) map[string]string {
	if c == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range c.Defaults.Headers {
		out[k] = v
	}
	for _, e := range c.Endpoints {
		if e.URL == url {
			for k, v := range e.Headers {
				out[k] = v
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
