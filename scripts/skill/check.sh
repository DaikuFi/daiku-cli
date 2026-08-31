#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/daiku-skill-check.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

DAIKU_SKILL_REFERENCE_DIR="$tmp" "$root/scripts/skill/generate.sh"
cmp "$tmp/commands.json" "$root/skills/daiku/references/commands.json"
cmp "$tmp/commands.md" "$root/skills/daiku/references/commands.md"
"$root/scripts/skill/install-test.sh"
printf '%s\n' 'skill checks passed'
