#!/usr/bin/env bash
# bump-version.sh — compute the next semver tag from git history.
#
# Usage:
#   ./scripts/bump-version.sh [major|minor|patch] [--dry-run]
#
# Prints the next tag (e.g. v1.2.3) to stdout. With --dry-run also prints
# current → next on stderr. Does not create tags or push.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BUMP="patch"
DRY_RUN=0

for arg in "$@"; do
  case "$arg" in
    major|minor|patch) BUMP="$arg" ;;
    --dry-run|-n) DRY_RUN=1 ;;
    -h|--help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      echo "usage: $0 [major|minor|patch] [--dry-run]" >&2
      exit 1
      ;;
  esac
done

latest="$(git describe --tags --abbrev=0 --match='v*' 2>/dev/null || true)"
if [[ -z "$latest" ]]; then
  current="0.0.0"
else
  current="${latest#v}"
fi

IFS='.' read -r major minor patch <<<"$current"
major="${major:-0}"
minor="${minor:-0}"
patch="${patch:-0}"
# strip any pre-release / build metadata from patch
patch="${patch%%[-+]*}"

case "$BUMP" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
esac

next="v${major}.${minor}.${patch}"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "current: v${current}" >&2
  echo "bump:    ${BUMP}" >&2
  echo "next:    ${next}" >&2
fi

echo "$next"
