#!/bin/sh
# Computes the next release version from the commits since the latest stable
# tag and prints it, v-prefixed, on stdout. Diagnostics go to stderr so the
# caller can capture stdout directly.
set -eu

usage() {
  cat >&2 <<'USAGE'
usage: version.sh [--bump auto|patch|minor|major] [--prerelease IDENTIFIER]

  --bump        How to increase the version. Default auto, which reads
                conventional commit prefixes since the latest stable tag:
                a "!" marker or a BREAKING CHANGE trailer selects major,
                feat selects minor, and anything else selects patch.
  --prerelease  Append a prerelease identifier such as rc, continuing the
                numbering of any existing prerelease for the same version.
USAGE
  exit 2
}

bump=auto
prerelease=
while [ "$#" -gt 0 ]; do
  case $1 in
    --bump) [ "$#" -ge 2 ] || usage; bump=$2; shift 2 ;;
    --prerelease) [ "$#" -ge 2 ] || usage; prerelease=$2; shift 2 ;;
    *) usage ;;
  esac
done

case $bump in auto|patch|minor|major) ;; *) usage ;; esac
if [ -n "$prerelease" ]; then
  printf '%s\n' "$prerelease" | grep -Eq '^[0-9A-Za-z-]+$' ||
    { printf 'version: prerelease identifier must be alphanumeric\n' >&2; exit 2; }
fi

# Only stable tags establish the base version. Prerelease tags are numbered
# from the stable version they lead to, so they must not become that base.
latest=$(git tag --list 'v*' |
  grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' |
  sort -t. -k1.2,1n -k2,2n -k3,3n |
  tail -n 1) || latest=

if [ -z "$latest" ]; then
  # No stable release yet, so there is nothing to increase from.
  major=0; minor=1; patch=0
  printf 'version: no stable tag found, starting at v0.1.0\n' >&2
else
  base=${latest#v}
  major=${base%%.*}
  rest=${base#*.}
  minor=${rest%%.*}
  patch=${rest#*.}

  if [ -z "$(git log --no-merges --format=%H "$latest..HEAD")" ]; then
    printf 'version: no commits since %s\n' "$latest" >&2
    exit 1
  fi

  if [ "$bump" = auto ]; then
    subjects=$(git log --no-merges --format=%s "$latest..HEAD")
    trailers=$(git log --no-merges --format=%b "$latest..HEAD")
    if printf '%s\n' "$subjects" | grep -qE '^[a-z]+(\([^)]*\))?!:' ||
       printf '%s\n' "$trailers" | grep -q '^BREAKING[ -]CHANGE'; then
      bump=major
      if [ "$major" -eq 0 ]; then
        # A 0.x line is still unstable, so a breaking change increases the
        # minor version. Reaching 1.0.0 stays an explicit, human decision.
        printf 'version: breaking change on a 0.x line, bumping minor\n' >&2
        bump=minor
      fi
    elif printf '%s\n' "$subjects" | grep -qE '^feat(\([^)]*\))?!?:'; then
      bump=minor
    else
      bump=patch
    fi
    printf 'version: detected %s bump from commits since %s\n' "$bump" "$latest" >&2
  fi

  case $bump in
    major) major=$((major + 1)); minor=0; patch=0 ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
  esac
fi

next="$major.$minor.$patch"

if [ -n "$prerelease" ]; then
  # Continue the existing prerelease series for this version rather than
  # restarting it, so rc.1 is not published twice.
  highest=$(git tag --list "v$next-$prerelease.*" |
    sed -n "s/^v$next-$prerelease\.\([0-9][0-9]*\)$/\1/p" |
    sort -n | tail -n 1) || highest=
  next="$next-$prerelease.$(( ${highest:-0} + 1 ))"
fi

"$(dirname "$0")/../install/daiku.sh" --validate-release-version "v$next" ||
  { printf 'version: computed an invalid version: v%s\n' "$next" >&2; exit 1; }

if git rev-parse -q --verify "refs/tags/v$next" >/dev/null 2>&1; then
  printf 'version: tag v%s already exists\n' "$next" >&2
  exit 1
fi

printf 'v%s\n' "$next"
