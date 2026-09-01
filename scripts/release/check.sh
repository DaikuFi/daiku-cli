#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

printf 'release-check: configuration contracts\n'
grep -q '^version: 2$' .goreleaser.yaml
grep -q 'draft: true' .goreleaser.yaml
grep -q 'prerelease: auto' .goreleaser.yaml
grep -q 'darwin, linux' .goreleaser.yaml
grep -q 'amd64, arm64' .goreleaser.yaml
grep -q 'cosign' .goreleaser.yaml
# The installer requires archives whose only member is the daiku binary, so the
# archive must not carry extra files. GoReleaser adds LICENSE/README globs
# unless a glob matching nothing suppresses them.
grep -q 'files: \[none\*\]' .goreleaser.yaml
test -f LICENSE
test -f NOTICE
# The draft workflow must accept stable versions; only stable releases reach
# the Homebrew tap, so a prerelease-only gate would strand the tap workflow.
grep -q 'validate-release-version' .github/workflows/release.yml
grep -q 'validate-release-version' .github/workflows/publish-tap.yml
sh -n scripts/install/daiku.sh scripts/install/test.sh scripts/release/homebrew.sh \
  scripts/release/version.sh scripts/release/artifact-check.sh \
  scripts/release/artifact-validate.sh scripts/release/artifact-validate-test.sh

printf 'release-check: artifact validator behavior\n'
./scripts/release/artifact-validate-test.sh

if [ "${CI:-}" = true ]; then
  printf 'release-check: GoReleaser configuration\n'
  goreleaser check
fi

printf 'release-check: installer behavior\n'
./scripts/install/test.sh

printf 'release-check: Homebrew generation\n'
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
if grep -q '{{' "$formula"; then
  echo 'Homebrew formula contains an unexpanded template value' >&2
  exit 1
fi

if ./scripts/release/homebrew.sh v01.2.3-rc.1 "$fixture" "$formula" >/dev/null 2>&1; then
  echo 'Homebrew generator accepted malformed version' >&2
  exit 1
fi
if DAIKU_RELEASE_REPOSITORY='owner/repo&unsafe' ./scripts/release/homebrew.sh v1.2.3-rc.1 "$fixture" "$formula" >/dev/null 2>&1; then
  echo 'Homebrew generator accepted malformed repository' >&2
  exit 1
fi
duplicate_checksum=$(head -n 1 "$fixture")
printf '%s\n' "$duplicate_checksum" >> "$fixture"
if ./scripts/release/homebrew.sh v1.2.3-rc.1 "$fixture" "$formula" >/dev/null 2>&1; then
  echo 'Homebrew generator accepted duplicate checksum' >&2
  exit 1
fi

printf 'release-check: Homebrew stable generation\n'
stable_fixture=$(mktemp "${TMPDIR:-/tmp}/daiku-checksums.XXXXXX")
trap 'rm -f "$fixture" "$formula" "$stable_fixture"' EXIT HUP INT TERM
for os in darwin linux; do
  for arch in amd64 arm64; do
    printf '%064d  daiku_1.2.3_%s_%s.tar.gz\n' 0 "$os" "$arch" >> "$stable_fixture"
  done
done
./scripts/release/homebrew.sh v1.2.3 "$stable_fixture" "$formula"
grep -q 'version "1.2.3"' "$formula"
grep -q 'license "Apache-2.0"' "$formula"
test "$(grep -c 'https://github.com/DaikuFi/daiku-cli/releases/download/v1.2.3/' "$formula")" -eq 4
if grep -q '{{' "$formula"; then
  echo 'Homebrew formula contains an unexpanded template value' >&2
  exit 1
fi

# The generator normalizes an optional leading "v", so bare 1.2.3 is valid here.
for bad in v1.2 v1.2.3.4 v01.2.3 v1.2.3- 'v1.2.3 rc'; do
  if ./scripts/release/homebrew.sh "$bad" "$stable_fixture" "$formula" >/dev/null 2>&1; then
    echo "Homebrew generator accepted malformed version: $bad" >&2
    exit 1
  fi
done

printf 'release-check: installer version validation\n'
for good in v1.2.3 v1.2.3-rc.1 v0.0.1 v10.20.30-alpha.1; do
  ./scripts/install/daiku.sh --validate-release-version "$good" || {
    echo "installer rejected valid release version: $good" >&2
    exit 1
  }
done
for bad in v1.2 v1.2.3.4 v01.2.3 1.2.3 v1.2.3-; do
  if ./scripts/install/daiku.sh --validate-release-version "$bad" >/dev/null 2>&1; then
    echo "installer accepted malformed release version: $bad" >&2
    exit 1
  fi
done
# The draft workflow still requires an explicit prerelease suffix.
if ./scripts/install/daiku.sh --validate-version v1.2.3 >/dev/null 2>&1; then
  echo 'draft validation accepted a stable version' >&2
  exit 1
fi
./scripts/install/daiku.sh --validate-version v1.2.3-rc.1

printf 'release-check: version computation\n'
version_repo=$(mktemp -d "${TMPDIR:-/tmp}/daiku-version.XXXXXX")
cleanup_version() { rm -rf "$version_repo"; }
trap 'rm -f "$fixture" "$formula" "$stable_fixture"; cleanup_version' EXIT HUP INT TERM
mkdir -p "$version_repo/scripts/release" "$version_repo/scripts/install"
cp scripts/release/version.sh "$version_repo/scripts/release/version.sh"
cp scripts/install/daiku.sh "$version_repo/scripts/install/daiku.sh"
(
  cd "$version_repo"
  git init -q .
  git config user.email release-check@example.invalid
  git config user.name release-check
  git commit -q --allow-empty -m 'initial'

  expect() {
    want=$1; shift
    got=$(./scripts/release/version.sh "$@" 2>/dev/null) || got='(refused)'
    if [ "$got" != "$want" ]; then
      echo "version.sh $*: expected $want, got $got" >&2
      exit 1
    fi
  }

  # With no stable tag the first release is v0.1.0.
  expect v0.1.0 --bump auto

  git tag v0.1.0
  git commit -q --allow-empty -m 'fix: a bug'
  expect v0.1.1 --bump auto
  expect v0.2.0 --bump minor
  expect v0.1.1-rc.1 --bump auto --prerelease rc

  git commit -q --allow-empty -m 'feat: a feature'
  expect v0.2.0 --bump auto

  # A breaking change on a 0.x line bumps minor; 1.0.0 stays explicit.
  git commit -q --allow-empty -m 'feat!: breaking'
  expect v0.2.0 --bump auto
  expect v1.0.0 --bump major

  # Version ordering must be numeric, so v0.10.0 outranks v0.9.0.
  git tag v0.9.0
  git tag v0.10.0
  git commit -q --allow-empty -m 'fix: another'
  expect v0.10.1 --bump auto

  # A prerelease series continues rather than restarting.
  git tag v0.10.1-rc.1
  expect v0.10.1-rc.2 --bump auto --prerelease rc

  # Nothing to release once HEAD is already tagged.
  git tag v0.10.1
  expect '(refused)' --bump auto

  # Malformed arguments are refused.
  expect '(refused)' --bump sideways
  expect '(refused)' --prerelease 'rc;rm'
) || exit 1
