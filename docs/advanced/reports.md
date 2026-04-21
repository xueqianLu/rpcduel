# Reports (HTML / Markdown / JUnit)

`replay` (and the other commands when applicable) can emit machine- and human-friendly reports in
addition to stdout.

| Flag | Format | Notes |
|---|---|---|
| `--report` | Text or JSON (matches `--output`) | Mirror of stdout to a file |
| `--csv` | CSV | Full per-diff detail (`category`, `method`, `params`, `detail`) |
| `--report-html` | Self-contained HTML | Includes a deterministic per-category bar chart |
| `--report-md` | Markdown | Includes a per-category share column with █ blocks |
| `--report-junit` | JUnit XML | One suite for threshold metrics + one suite per diff category |

Default destinations can be set under the `reports:` section of the [config file](/advanced/config),
and per-flag CLI values override them.

## HTML

Self-contained single file with no external CSS/JS. Includes a sorted-by-category bar chart so the
distribution is reviewable at a glance.

## Markdown

Human-readable summary plus a per-category share column with `█` blocks. Great for posting into
Slack, GitHub PR comments, or release notes.

## JUnit

Two suites:

| Suite | Contents |
|---|---|
| `replay` | One `<testcase>` per threshold metric (`mismatch_rate`, `error_rate`, `max_mismatch`). Breaches become `<failure>` nodes. |
| `replay_categories` | One informational `<testcase>` per diff category — surfaces the breakdown inline in CI test reporters. |

Pair with `dorny/test-reporter` (GitHub Actions) or `artifacts:reports:junit` (GitLab CI) — see the
[CI templates](/advanced/ci).

## Example

```bash
rpcduel replay \
  --dataset dataset.json \
  --rpc $NODE_A --rpc $NODE_B \
  --report replay.json --output json \
  --csv replay.csv \
  --report-html replay.html \
  --report-md replay.md \
  --report-junit replay.junit.xml
```
