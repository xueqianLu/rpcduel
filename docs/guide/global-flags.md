# Global Flags

These flags apply to **every** subcommand.

| Flag | Default | Description |
|------|---------|-------------|
| `--config`, `-c` | (none) | Path to an `rpcduel.yaml` config file. CLI flags always override config values. See [Configuration file](/advanced/config). |
| `--log-level` | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |
| `--log-format` | `text` | Log format: `text` or `json` (structured output via `slog`). |
| `--retries` | `0` | Retries on network / 5xx / 408 / 429 failures. JSON-RPC application errors are not retried. |
| `--retry-backoff` | `200ms` | Initial exponential backoff between retries. |
| `--header` | (none) | Extra HTTP header sent with every RPC request. May be repeated. Accepts `Key: Value` or `Key=Value`. |
| `--user-agent` | `rpcduel/<version>` | Override the `User-Agent` header. |
| `--insecure` | `false` | Skip TLS certificate verification. **Development only.** |
| `--metrics-addr` | (disabled) | Expose Prometheus metrics at `/metrics` on the given address (e.g. `:9090`). |
| `--push-gateway` | (disabled) | Prometheus Pushgateway URL; metrics are pushed at command exit. |
| `--push-job` | `rpcduel` | Job label used when pushing. |
| `--push-label` | (none) | Extra Pushgateway grouping label, repeatable, in `key=value` form. |

::: tip
For long-lived runs, prefer `--metrics-addr` and let Prometheus pull. For short, ad-hoc CI jobs, use
`--push-gateway` so the metrics actually land somewhere before the process exits.
:::

See also: [Configuration file](/advanced/config) · [Prometheus & Pushgateway](/advanced/metrics)
