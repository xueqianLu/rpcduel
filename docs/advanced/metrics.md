# Prometheus Metrics & Pushgateway

rpcduel exposes the same metric registry over **two transports**: a long-lived HTTP scrape endpoint
for `prometheus pull`, and a Pushgateway client for short-lived CI / cron jobs.

## Pull mode — `--metrics-addr`

Pass `--metrics-addr :9090` to any command to expose a Prometheus-format endpoint at
`http://localhost:9090/metrics` while the command runs.

## Push mode — `--push-gateway`

For short-lived runs (CI, cron) the process exits before Prometheus has a chance to scrape. Use the
Pushgateway flags instead — metrics are pushed once at command exit.

```bash
rpcduel replay \
  --dataset dataset.json \
  --rpc https://a.example.com --rpc https://b.example.com \
  --push-gateway http://pushgateway:9091 \
  --push-job nightly-replay \
  --push-label run_id=$GITHUB_RUN_ID \
  --push-label chain=mainnet
```

| Flag | Default | Description |
|---|---|---|
| `--push-gateway` | (disabled) | Pushgateway URL |
| `--push-job` | `rpcduel` | Job label |
| `--push-label` | (none) | Extra grouping label, repeatable, `key=value` |

## Published metrics

| Metric | Type | Labels |
|---|---|---|
| `rpcduel_requests_total` | counter | `endpoint`, `scenario`, `status` (`ok`/`error`) |
| `rpcduel_request_duration_seconds` | histogram | `endpoint`, `scenario` |
| `rpcduel_diffs_total` | counter | `endpoint_a`, `endpoint_b` |
| `rpcduel_replay_diffs_total` | counter | `category` |

`scenario` is the per-request tag from a bench scenario file when present, otherwise the JSON-RPC
method name.

## Example PromQL

```promql
# P95 latency by endpoint over the last 5 minutes
histogram_quantile(0.95,
  sum(rate(rpcduel_request_duration_seconds_bucket[5m])) by (le, endpoint)
)

# Replay diff rate per category, last run
sum(rpcduel_replay_diffs_total) by (category)
```
