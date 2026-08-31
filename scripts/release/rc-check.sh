#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

printf '%s\n' 'rc-check: formatting, contract, vet, race, and integration suites'
make check

printf '%s\n' 'rc-check: dependency vulnerability scan'
make security-check

printf '%s\n' 'rc-check: Agent Skill manifests and installers'
make skill-check

printf '%s\n' 'rc-check: release, installer, version, and Homebrew contracts'
make release-check

printf '%s\n' 'rc-check: macOS and Linux artifacts on amd64 and arm64'
make cross-build

printf '%s\n' 'rc-check: release candidate is ready for human approval'
