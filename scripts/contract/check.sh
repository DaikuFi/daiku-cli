#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$repo_root"

go run ./cmd/contractcheck

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/daiku-contract.XXXXXX")
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

mkdir -p "$tmp_dir/generated/daikuv1"
sed "s#^output: .*#output: $tmp_dir/generated/daikuv1/client.gen.go#" \
  openapi/oapi-codegen.yaml > "$tmp_dir/oapi-codegen.yaml"
go tool oapi-codegen --config "$tmp_dir/oapi-codegen.yaml" openapi/daiku-v1.json
gofmt -w "$tmp_dir/generated/daikuv1/client.gen.go"

if ! cmp -s generated/daikuv1/client.gen.go "$tmp_dir/generated/daikuv1/client.gen.go"; then
  echo "generated client is stale; run scripts/contract/generate.sh" >&2
  diff -u generated/daikuv1/client.gen.go "$tmp_dir/generated/daikuv1/client.gen.go" || true
  exit 1
fi
