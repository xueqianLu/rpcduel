# CI Templates

Drop-in pipelines for **GitHub Actions** and **GitLab CI** live under
[`examples/ci/`](https://github.com/xueqianLu/rpcduel/tree/main/examples/ci) in the repository.

They wire [`doctor`](/advanced/doctor) + [`diff`](/commands/diff) + [`replay`](/data-driven/replay)
with [SLO thresholds](/advanced/thresholds) and JUnit uploads.

## What the templates do

1. **Install rpcduel** from the GitHub release tarball
2. **`doctor`** the two endpoints first — fail fast on unreachable nodes (exit 2)
3. **`diff`** a few representative methods
4. **`replay`** the dataset (downloaded from artifacts or generated inline)
5. **Upload JUnit reports** to the test reporter

Both templates use the `rpcduel.yaml` from the repo so SLO thresholds are version-controlled
alongside code.

## Suggested layout

```
.github/workflows/rpc-conformance.yml   # copy of examples/ci/github-actions.yml
rpcduel.yaml                            # SLO thresholds, default flags
datasets/mainnet-small.json             # checked-in or built nightly
```

## See also

* [`examples/ci/github-actions.yml`](https://github.com/xueqianLu/rpcduel/blob/main/examples/ci/github-actions.yml)
* [`examples/ci/gitlab-ci.yml`](https://github.com/xueqianLu/rpcduel/blob/main/examples/ci/gitlab-ci.yml)
* [`examples/ci/README.md`](https://github.com/xueqianLu/rpcduel/blob/main/examples/ci/README.md)
* [Reports](/advanced/reports) — JUnit / HTML / Markdown formats
