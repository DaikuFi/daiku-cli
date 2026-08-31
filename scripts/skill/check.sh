#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/daiku-skill-check.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

DAIKU_SKILL_REFERENCE_DIR="$tmp/references" \
DAIKU_SKILL_INTEGRITY_PATH="$tmp/integrity.json" \
DAIKU_SKILL_GO_PATH="$tmp/integrity_generated.go" \
  "$root/scripts/skill/generate.sh"
cmp "$tmp/references/commands.json" "$root/skills/daiku/references/commands.json"
cmp "$tmp/references/commands.md" "$root/skills/daiku/references/commands.md"
cmp "$tmp/integrity.json" "$root/skills/daiku/integrity.json"
cmp "$tmp/integrity_generated.go" "$root/internal/skillmeta/integrity_generated.go"
"$root/scripts/skill/install-test.sh"
printf '%s\n' 'skill checks passed'
