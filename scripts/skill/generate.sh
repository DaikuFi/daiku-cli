#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/daiku-skill-generate.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
output_dir=${DAIKU_SKILL_REFERENCE_DIR:-"$root/skills/daiku/references"}
mkdir -p "$output_dir"

(cd "$root" && GOCACHE=${GOCACHE:-"$tmp/go-cache"} go build -o "$tmp/daiku" ./cmd/daiku)
"$tmp/daiku" commands --agent > "$tmp/commands-envelope.json"
GOCACHE=${GOCACHE:-"$tmp/go-cache"} go run "$root/scripts/skill/render.go" \
  "$tmp/commands-envelope.json" \
  "$output_dir/commands.json" \
  "$output_dir/commands.md"
