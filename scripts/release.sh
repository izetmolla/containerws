#!/usr/bin/env bash
# release.sh — bump version, update CHANGELOG, create annotated tag, push to GitHub.
#
# Pushing a v* tag triggers .github/workflows/release.yml which builds the
# frontend, runs GoReleaser (binaries only — no Docker images), and publishes
# a GitHub Release.
#
# Usage:
#   ./scripts/release.sh [major|minor|patch]
#   ./scripts/release.sh patch --no-push   # tag locally only
#   ./scripts/release.sh minor --yes       # skip confirmation
#
# Prerequisites: clean git working tree, push access to origin.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BUMP="patch"
PUSH=1
YES=0

for arg in "$@"; do
  case "$arg" in
    major|minor|patch) BUMP="$arg" ;;
    --no-push) PUSH=0 ;;
    --yes|-y) YES=1 ;;
    -h|--help)
      sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      echo "usage: $0 [major|minor|patch] [--no-push] [--yes]" >&2
      exit 1
      ;;
  esac
done

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree is not clean; commit or stash first" >&2
  git status --short >&2
  exit 1
fi

git fetch --tags --quiet 2>/dev/null || true

NEXT="$("$ROOT/scripts/bump-version.sh" "$BUMP")"
CURRENT="$(git describe --tags --abbrev=0 --match='v*' 2>/dev/null || echo 'v0.0.0')"
VERSION="${NEXT#v}"
DATE="$(date -u +%Y-%m-%d)"

echo "Release plan"
echo "  bump:    ${BUMP}"
echo "  current: ${CURRENT}"
echo "  next:    ${NEXT}"
echo "  push:    $([[ "$PUSH" -eq 1 ]] && echo yes || echo no)"
echo

if [[ "$YES" -ne 1 ]]; then
  read -r -p "Create release ${NEXT}? [y/N] " reply
  case "$reply" in
    y|Y|yes|YES) ;;
    *) echo "aborted"; exit 1 ;;
  esac
fi

# Prefer commit-and-tag-version when available (same tool as filebrowser).
if command -v pnpm >/dev/null 2>&1; then
  RELEASE_AS="$VERSION" pnpm dlx commit-and-tag-version \
    --release-as "$VERSION" \
    --skip.bump false \
    --skip.changelog false \
    --skip.commit false \
    --skip.tag false \
    -s
else
  {
    echo "## [${VERSION}] - ${DATE}"
    echo
    echo "### Changed"
    echo
    echo "- Release ${NEXT}"
    echo
  } >"${ROOT}/.changelog-entry.tmp"

  if [[ -f CHANGELOG.md ]]; then
    {
      head -n 7 CHANGELOG.md
      cat "${ROOT}/.changelog-entry.tmp"
      tail -n +8 CHANGELOG.md
    } >"${ROOT}/CHANGELOG.md.tmp"
    mv "${ROOT}/CHANGELOG.md.tmp" CHANGELOG.md
  else
    mv "${ROOT}/.changelog-entry.tmp" CHANGELOG.md
  fi
  rm -f "${ROOT}/.changelog-entry.tmp"

  git add CHANGELOG.md
  git commit -m "chore(release): ${NEXT}"
  git tag -a "$NEXT" -m "Release ${NEXT}"
fi

# Ensure the annotated tag exists (commit-and-tag-version creates it).
if ! git rev-parse "$NEXT" >/dev/null 2>&1; then
  git tag -a "$NEXT" -m "Release ${NEXT}"
fi

echo "Created tag ${NEXT}"

if [[ "$PUSH" -eq 1 ]]; then
  BRANCH="$(git rev-parse --abbrev-ref HEAD)"
  echo "Pushing ${BRANCH} and tag ${NEXT} to origin…"
  git push origin "$BRANCH"
  git push origin "$NEXT"
  echo
  echo "GitHub Actions will build binaries and publish the release."
  REMOTE_URL="$(git remote get-url origin 2>/dev/null || true)"
  if [[ "$REMOTE_URL" =~ github.com[:/]([^/]+)/([^/.]+) ]]; then
    echo "Track: https://github.com/${BASH_REMATCH[1]}/${BASH_REMATCH[2]}/actions"
  fi
else
  echo "Skipped push (--no-push). When ready:"
  echo "  git push origin HEAD && git push origin ${NEXT}"
fi
