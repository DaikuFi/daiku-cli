#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

command -v goreleaser >/dev/null 2>&1 || {
  echo 'artifact-check: goreleaser is required' >&2
  exit 1
}

goreleaser release --snapshot --clean --skip=sign,sbom,publish

version=$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' dist/metadata.json)
./scripts/release/artifact-validate.sh dist "$version"

printf '%s\n' 'artifact checks passed'
