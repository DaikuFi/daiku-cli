#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
validator="$root/scripts/release/artifact-validate.sh"
fixture=$(mktemp -d "${TMPDIR:-/tmp}/daiku-artifacts.XXXXXX")
trap 'rm -rf "$fixture"' EXIT HUP INT TERM

make_fixture() {
  rm -rf "$fixture/dist" "$fixture/payload"
  mkdir -p "$fixture/dist" "$fixture/payload"
  printf '%s\n' daiku > "$fixture/payload/daiku"
  for os in darwin linux; do
    for arch in amd64 arm64; do
      archive="daiku_1.2.3_${os}_${arch}.tar.gz"
      tar -czf "$fixture/dist/$archive" -C "$fixture/payload" daiku
      shasum -a 256 "$fixture/dist/$archive" | awk '{ print $1 "  " name }' name="$archive" >> "$fixture/dist/checksums.txt"
    done
  done
}

expect_failure() {
  label=$1
  if "$validator" "$fixture/dist" 1.2.3 >/dev/null 2>&1; then
    echo "artifact validator accepted $label" >&2
    exit 1
  fi
}

make_fixture
"$validator" "$fixture/dist" 1.2.3 >/dev/null

make_fixture
cp "$fixture/dist/daiku_1.2.3_linux_amd64.tar.gz" "$fixture/dist/unexpected.tar.gz"
expect_failure 'an extra top-level archive'

make_fixture
rm "$fixture/dist/daiku_1.2.3_linux_amd64.tar.gz"
expect_failure 'a missing top-level archive'

make_fixture
duplicate=$(head -n 1 "$fixture/dist/checksums.txt")
printf '%s\n' "$duplicate" >> "$fixture/dist/checksums.txt"
expect_failure 'a duplicate checksum filename'

make_fixture
sed -i.bak '/linux_amd64/d' "$fixture/dist/checksums.txt"
rm "$fixture/dist/checksums.txt.bak"
expect_failure 'a missing checksum filename'

make_fixture
printf '%064d  unexpected.tar.gz\n' 0 >> "$fixture/dist/checksums.txt"
expect_failure 'an extra checksum filename'

printf '%s\n' 'artifact validator tests passed'
