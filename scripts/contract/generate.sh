#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$repo_root"

go run ./cmd/contractcheck
mkdir -p generated/daikuv1
go tool oapi-codegen --config openapi/oapi-codegen.yaml openapi/daiku-v1.json
gofmt -w generated/daikuv1/client.gen.go
