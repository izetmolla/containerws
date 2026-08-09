# ContainerWS documentation

Product docs site built with [Docusaurus](https://docusaurus.io/docs).

**Published site:** [https://containerws.izetmolla.com](https://containerws.izetmolla.com)

Hosted on GitHub Pages for `izetmolla/containerws` with custom domain `containerws.izetmolla.com`.

## DNS (one-time)

At your DNS provider for `izetmolla.com`, add:

| Type  | Name           | Target / value        |
|-------|----------------|-----------------------|
| CNAME | `containerws`  | `izetmolla.github.io` |

Then in the repo **Settings → Pages → Custom domain**, set `containerws.izetmolla.com` and enable **Enforce HTTPS** after DNS verifies.

## Develop

```bash
cd docs
pnpm install
pnpm start
```

Or from the repo root: `task docs:start`

Local full-text search (navbar, `Ctrl/Cmd+K`) indexes all docs and pages via `@easyops-cn/docusaurus-search-local` — no Algolia account required.

## Build

```bash
pnpm build
# or: task docs:build
```

## Publish

Pushing `docs/` changes to `main` runs [`.github/workflows/docs.yml`](../.github/workflows/docs.yml). Manual: **Actions → Deploy docs → Run workflow**.
