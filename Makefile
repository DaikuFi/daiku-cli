.PHONY: build test check cross-build

build:
	go build -o bin/daiku ./cmd/daiku

test:
	go test ./...

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test -race ./...

cross-build:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/daiku-darwin-amd64 ./cmd/daiku
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/daiku-darwin-arm64 ./cmd/daiku
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/daiku-linux-amd64 ./cmd/daiku
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/daiku-linux-arm64 ./cmd/daiku
