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
printf '%s\n' "$version" | grep -Eq '^[0-9A-Za-z.-]+$' || {
  echo 'artifact-check: invalid snapshot version metadata' >&2
  exit 1
}

expected=$(printf '%s\n' \
  "daiku_${version}_darwin_amd64.tar.gz" \
  "daiku_${version}_darwin_arm64.tar.gz" \
  "daiku_${version}_linux_amd64.tar.gz" \
  "daiku_${version}_linux_arm64.tar.gz" | sort)
actual=$(find dist -maxdepth 1 -type f -name 'daiku_*.tar.gz' -exec basename {} \; | sort)
if [ "$actual" != "$expected" ]; then
  echo 'artifact-check: release archive set does not match supported targets' >&2
  printf '%s\n' 'expected:' "$expected" 'actual:' "$actual" >&2
  exit 1
fi

for archive in $expected; do
  archive="dist/$archive"
  test "$(tar -tzf "$archive")" = daiku || {
    echo "artifact-check: $archive must contain only the daiku binary" >&2
    exit 1
  }
done

test -f dist/checksums.txt
test "$(wc -l < dist/checksums.txt | tr -d ' ')" -eq 4
(cd dist && shasum -a 256 -c checksums.txt)

printf '%s\n' 'artifact checks passed'
