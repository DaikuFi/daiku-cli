package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/DaikuFi/daiku-cli/internal/contract"
)

func main() {
	schema := flag.String("schema", "openapi/daiku-v1.json", "pinned OpenAPI schema")
	operations := flag.String("operations", "openapi/operation-ids.txt", "accepted operationId manifest")
	checksum := flag.String("checksum", "openapi/daiku-v1.sha256", "schema checksum")
	source := flag.String("source", "openapi/SOURCE.json", "schema provenance")
	flag.Parse()

	if err := contract.Verify(*schema, *operations, *checksum, *source); err != nil {
		fmt.Fprintf(os.Stderr, "contract check failed: %v\n", err)
		os.Exit(1)
	}
}
