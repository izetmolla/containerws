# ContainerWS documentation

Product docs site built with [Docusaurus](https://docusaurus.io/docs).

**Published site:** [https://izetmolla.github.io/containerws/](https://izetmolla.github.io/containerws/)

GitHub Pages hosts project sites under `https://<user>.github.io/<repo>/` (no free `containerws.*` subdomain unless you add a custom domain or a GitHub org named `containerws`).

## Develop

```bash
cd docs
pnpm install
pnpm start
```

Or from the repo root: `task docs:start`

## Build

```bash
pnpm build
# or: task docs:build
```

## Publish

Pushing changes under `docs/` to `main` runs [`.github/workflows/docs.yml`](../.github/workflows/docs.yml) and deploys to GitHub Pages. Manual run: **Actions → Deploy docs → Run workflow**.
