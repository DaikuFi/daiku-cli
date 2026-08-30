package contract_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/contract"
)

const validSchema = `{
  "openapi": "3.0.3",
  "paths": {
    "/api/v1/things/": {
      "get": {
        "operationId": "daiku_things_get",
        "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Thing"}}}}}
      }
    }
  },
  "components": {"schemas": {"Thing": {"type": "object", "properties": {"id": {"type": "string"}}}}}
}`

func TestVerifyAcceptsPinnedContract(t *testing.T) {
	root := repositoryRoot(t)
	err := contract.Verify(
		filepath.Join(root, "openapi/daiku-v1.json"),
		filepath.Join(root, "openapi/operations.txt"),
		filepath.Join(root, "openapi/daiku-v1.sha256"),
		filepath.Join(root, "openapi/SOURCE.json"),
	)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyRejectsMissingOperationID(t *testing.T) {
	schema := strings.Replace(validSchema, `"operationId": "daiku_things_get",`, "", 1)
	paths := writeFixture(t, schema, "GET /api/v1/things/ daiku_things_get\n")

	err := contract.Verify(paths.schema, paths.operations, paths.checksum, paths.source)
	if err == nil || !strings.Contains(err.Error(), "has no operationId") {
		t.Fatalf("Verify() error = %v, want missing operationId", err)
	}
}

func TestVerifyRejectsChangedOperationID(t *testing.T) {
	schema := strings.Replace(validSchema, "daiku_things_get", "daiku_things_list", 1)
	paths := writeFixture(t, schema, "GET /api/v1/things/ daiku_things_get\n")

	err := contract.Verify(paths.schema, paths.operations, paths.checksum, paths.source)
	if err == nil || !strings.Contains(err.Error(), "incompatible operation surface") {
		t.Fatalf("Verify() error = %v, want incompatible operation surface", err)
	}
}

func TestVerifyRejectsOperationMovedWithoutRename(t *testing.T) {
	schema := strings.Replace(validSchema, `"/api/v1/things/"`, `"/api/v1/archived-things/"`, 1)
	paths := writeFixture(t, schema, "GET /api/v1/things/ daiku_things_get\n")

	err := contract.Verify(paths.schema, paths.operations, paths.checksum, paths.source)
	if err == nil || !strings.Contains(err.Error(), "incompatible operation surface") ||
		!strings.Contains(err.Error(), "GET /api/v1/things/ daiku_things_get") ||
		!strings.Contains(err.Error(), "GET /api/v1/archived-things/ daiku_things_get") {
		t.Fatalf("Verify() error = %v, want old and moved operation signatures", err)
	}
}

func TestVerifyRejectsUnresolvedSchemaReference(t *testing.T) {
	schema := strings.Replace(validSchema, "#/components/schemas/Thing", "#/components/schemas/Missing", 1)
	paths := writeFixture(t, schema, "GET /api/v1/things/ daiku_things_get\n")

	err := contract.Verify(paths.schema, paths.operations, paths.checksum, paths.source)
	if err == nil || !strings.Contains(err.Error(), "unresolved schema reference") {
		t.Fatalf("Verify() error = %v, want unresolved reference", err)
	}
}

func TestVerifyRejectsChecksumDrift(t *testing.T) {
	paths := writeFixture(t, validSchema, "GET /api/v1/things/ daiku_things_get\n")
	if err := os.WriteFile(paths.schema, []byte(validSchema+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := contract.Verify(paths.schema, paths.operations, paths.checksum, paths.source)
	if err == nil || !strings.Contains(err.Error(), "schema checksum mismatch") {
		t.Fatalf("Verify() error = %v, want checksum mismatch", err)
	}
}

type fixturePaths struct {
	schema, operations, checksum, source string
}

func writeFixture(t *testing.T, schema, operations string) fixturePaths {
	t.Helper()
	dir := t.TempDir()
	paths := fixturePaths{
		schema:     filepath.Join(dir, "daiku-v1.json"),
		operations: filepath.Join(dir, "operations.txt"),
		checksum:   filepath.Join(dir, "daiku-v1.sha256"),
		source:     filepath.Join(dir, "SOURCE.json"),
	}
	digest := sha256.Sum256([]byte(schema))
	files := map[string]string{
		paths.schema:     schema,
		paths.operations: operations,
		paths.checksum:   fmt.Sprintf("%x  daiku-v1.json\n", digest),
		paths.source:     fmt.Sprintf(`{"repository":"https://github.com/DaikuFi/Daiku","commit":"0b7f71442e6b1f0bf67f149158385ef7a9c49266","schema":"openapi/daiku-v1.json","sha256":"%x"}`, digest),
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}
