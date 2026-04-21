# SLO Thresholds & CI Gating

Declare SLO thresholds in the [config file](/advanced/config) so any breach prints a `FAIL`
summary to stderr and **exits with code 2** — your CI fails loudly without extra scripting.

## Schema

```yaml
thresholds:
  bench:
    p95_ms: 250         # P95 latency in milliseconds
    p99_ms: 500
    error_rate: 0.01    # 1%
  duel:
    p99_ms: 500
    error_rate: 0.01
    diff_rate: 0.001    # 0.1% mismatch
  diff:
    diff_rate: 0.0
    max_diffs: 0
  replay:
    mismatch_rate: 0.001
    error_rate: 0.01
    max_mismatch: 5
```

Zero / unset means "do not check".

## Exit codes

| Code | Meaning |
|---|---|
| `0` | OK (no thresholds breached, or none configured) |
| `1` | Generic CLI / runtime error |
| `2` | One or more thresholds breached **or** [`doctor`](/advanced/doctor) detected unhealthy endpoints |
| `130` | Interrupted (Ctrl-C) |

## Output

A `FAIL` summary is emitted to **stderr** and the structured / JUnit reports include a
`thresholds` block that lists every breach with the actual measured value:

```
FAIL: replay.mismatch_rate=0.0042 (>= 0.001)
FAIL: replay.error_rate=0.0150 (>= 0.01)
```

## CI integration tips

* Always pair thresholds with [`--report-junit`](/advanced/reports) so the breaches show up inline in GitHub / GitLab test reporters.
* Run [`doctor`](/advanced/doctor) **before** the gated command so unreachable endpoints fail fast (also exit 2) without consuming budget.
* Use the [Pushgateway flags](/advanced/metrics) so the run's metrics land in Prometheus even when the job fails.

## See also

* [Configuration file](/advanced/config)
* [Reports (HTML / MD / JUnit)](/advanced/reports)
* [CI templates](/advanced/ci)
