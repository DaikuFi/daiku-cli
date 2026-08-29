#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

grep -q '^version: 2$' .goreleaser.yaml
grep -q 'draft: true' .goreleaser.yaml
grep -q 'prerelease: auto' .goreleaser.yaml
grep -q 'darwin, linux' .goreleaser.yaml
grep -q 'amd64, arm64' .goreleaser.yaml
grep -q 'cosign' .goreleaser.yaml
sh -n scripts/install/daiku.sh scripts/install/test.sh scripts/release/homebrew.sh

if [ "${CI:-}" = true ]; then goreleaser check; fi

./scripts/install/test.sh

fixture=$(mktemp "${TMPDIR:-/tmp}/daiku-checksums.XXXXXX")
formula=$(mktemp "${TMPDIR:-/tmp}/daiku-formula.XXXXXX")
trap 'rm -f "$fixture" "$formula"' EXIT HUP INT TERM
for os in darwin linux; do
  for arch in amd64 arm64; do
    printf '%064d  daiku_1.2.3-rc.1_%s_%s.tar.gz\n' 0 "$os" "$arch" >> "$fixture"
  done
done
./scripts/release/homebrew.sh v1.2.3-rc.1 "$fixture" "$formula"
grep -q 'version "1.2.3-rc.1"' "$formula"
test "$(grep -c 'https://github.com/DaikuFi/daiku-cli/releases/download/' "$formula")" -eq 4
! grep -q '{{' "$formula"

if ./scripts/release/homebrew.sh v01.2.3-rc.1 "$fixture" "$formula" >/dev/null 2>&1; then
  echo 'Homebrew generator accepted malformed version' >&2
  exit 1
fi
if DAIKU_RELEASE_REPOSITORY='owner/repo&unsafe' ./scripts/release/homebrew.sh v1.2.3-rc.1 "$fixture" "$formula" >/dev/null 2>&1; then
  echo 'Homebrew generator accepted malformed repository' >&2
  exit 1
fi
head -n 1 "$fixture" >> "$fixture"
if ./scripts/release/homebrew.sh v1.2.3-rc.1 "$fixture" "$formula" >/dev/null 2>&1; then
  echo 'Homebrew generator accepted duplicate checksum' >&2
  exit 1
fi
