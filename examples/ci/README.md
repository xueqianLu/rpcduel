# CI integration examples

This directory contains ready-to-use CI templates that wire rpcduel
into a typical health + regression pipeline:

| File                 | Platform        | Notes                                          |
|----------------------|-----------------|------------------------------------------------|
| `github-actions.yml` | GitHub Actions  | Uses `dorny/test-reporter` for inline checks.  |
| `gitlab-ci.yml`      | GitLab CI/CD    | Uses the built-in `reports:junit` artifact.    |

Typical shape of either pipeline:

1. **`rpcduel doctor`** — fast liveness and capability probe. Fails the
   pipeline immediately if any endpoint is unreachable.
2. **`rpcduel diff`** — response-consistency gate, with
   `--max-diff-rate` as the SLO.
3. **`rpcduel replay`** — dataset-driven regression, with
   `--max-mismatch-rate` / `--max-error-rate` SLOs.
4. Every verb emits HTML (human) + JUnit (machine) reports. JUnit is
   rendered natively by GitHub checks and GitLab MR widgets.

All SLO breaches cause rpcduel to exit with code **2**, distinct from
generic errors (1) and signals (130), so CI step definitions can keep
`continue-on-error` semantics simple.
