# rpcduel documentation site

This folder hosts the [VitePress](https://vitepress.dev/) source for the rpcduel documentation site
published at <https://xueqianlu.github.io/rpcduel/>.

## Local development

```bash
cd docs
npm install
npm run docs:dev      # http://localhost:5173
```

## Build

```bash
npm run docs:build    # output in docs/.vitepress/dist
npm run docs:preview  # serve the built site locally
```

## Deployment

Pushing to `main` triggers `.github/workflows/docs.yml`, which builds the site and deploys it to
GitHub Pages. The site source lives next to the code on purpose: every PR that changes a flag or a
command can update the docs in the same review.

## Structure

```
docs/
├── .vitepress/config.ts       # site config + sidebar
├── public/                    # static assets (logo, favicon)
├── index.md                   # landing page
├── guide/                     # getting-started, install, global flags
├── commands/                  # call, diff, bench, duel
├── data-driven/               # ⭐ dataset, replay, benchgen, workflow
├── advanced/                  # config, thresholds, reports, metrics, doctor, ci, completions
└── reference/                 # output formats, dataset format, architecture
```

## Conventions

* One Markdown file per command / topic.
* Internal links use site-absolute paths (`/data-driven/replay`), no `.md` extension, so VitePress
  picks them up correctly with `cleanUrls`.
* Code samples should match the binary on `main`. When you change a flag in `cmd/`, update the
  corresponding doc page in the **same PR**.
