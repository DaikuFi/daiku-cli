#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

command -v goreleaser >/dev/null 2>&1 || {
  echo 'artifact-check: goreleaser is required' >&2
  exit 1
}

goreleaser release --snapshot --clean --skip=sign,sbom,publish

archives=$(find dist -maxdepth 1 -type f -name 'daiku_*.tar.gz' | sort)
test "$(printf '%s\n' "$archives" | grep -c .)" -eq 4 || {
  echo 'artifact-check: expected four release archives' >&2
  exit 1
}

for archive in $archives; do
  test "$(tar -tzf "$archive")" = daiku || {
    echo "artifact-check: $archive must contain only the daiku binary" >&2
    exit 1
  }
done

test -f dist/checksums.txt
test "$(wc -l < dist/checksums.txt | tr -d ' ')" -eq 4
(cd dist && shasum -a 256 -c checksums.txt)

printf '%s\n' 'artifact checks passed'
