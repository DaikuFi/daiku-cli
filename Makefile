.PHONY: build test check cross-build contract-generate contract-check release-check installer-test

build:
	go build -o bin/daiku ./cmd/daiku

test:
	go test ./...

check:
	test -z "$$(gofmt -l .)"
	./scripts/contract/check.sh
	go vet ./...
	go test -race ./...

contract-generate:
	./scripts/contract/generate.sh

contract-check:
	./scripts/contract/check.sh

cross-build:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/daiku-darwin-amd64 ./cmd/daiku
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/daiku-darwin-arm64 ./cmd/daiku
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/daiku-linux-amd64 ./cmd/daiku
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/daiku-linux-arm64 ./cmd/daiku

release-check:
	./scripts/release/check.sh

installer-test:
	./scripts/install/test.sh
