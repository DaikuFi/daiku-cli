package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

var httpMethods = map[string]struct{}{
	"delete": {}, "get": {}, "head": {}, "options": {},
	"patch": {}, "post": {}, "put": {}, "trace": {},
}

type provenance struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Schema     string `json:"schema"`
	SHA256     string `json:"sha256"`
}

// Verify validates the pinned schema, its provenance, checksum, references and
// exact method/path/operationId surface. Any contract update must intentionally update all
// four artifacts before generated code can change.
func Verify(schemaPath, operationsPath, checksumPath, provenancePath string) error {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	if err := verifyChecksum(schemaBytes, checksumPath); err != nil {
		return err
	}
	if err := verifyProvenance(schemaBytes, provenancePath); err != nil {
		return err
	}

	var document map[string]any
	if err := json.Unmarshal(schemaBytes, &document); err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}
	version, _ := document["openapi"].(string)
	if !strings.HasPrefix(version, "3.0.") {
		return fmt.Errorf("unsupported OpenAPI version %q; expected 3.0.x", version)
	}

	actual, err := validateOperations(document)
	if err != nil {
		return err
	}
	if err := validateReferences(document); err != nil {
		return err
	}
	expectedBytes, err := os.ReadFile(operationsPath)
	if err != nil {
		return fmt.Errorf("read operation manifest: %w", err)
	}
	expected := nonemptyLines(string(expectedBytes))
	if err := compareOperations(expected, actual); err != nil {
		return err
	}
	return nil
}

func verifyChecksum(schema []byte, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema checksum: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 || fields[1] != "daiku-v1.json" {
		return errors.New("checksum file must contain '<sha256>  daiku-v1.json'")
	}
	want, err := hex.DecodeString(fields[0])
	if err != nil || len(want) != sha256.Size {
		return errors.New("schema checksum is not a SHA-256 digest")
	}
	got := sha256.Sum256(schema)
	if !equalBytes(want, got[:]) {
		return fmt.Errorf("schema checksum mismatch: got %x, want %s", got, fields[0])
	}
	return nil
}

func verifyProvenance(schema []byte, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema provenance: %w", err)
	}
	var source provenance
	if err := json.Unmarshal(raw, &source); err != nil {
		return fmt.Errorf("parse schema provenance: %w", err)
	}
	if source.Repository != "https://github.com/DaikuFi/Daiku" {
		return fmt.Errorf("unexpected schema repository %q", source.Repository)
	}
	if len(source.Commit) != 40 || !isLowerHex(source.Commit) {
		return errors.New("schema provenance commit must be a full lowercase Git SHA")
	}
	if source.Schema != "openapi/daiku-v1.json" {
		return fmt.Errorf("unexpected provenance schema path %q", source.Schema)
	}
	digest := sha256.Sum256(schema)
	if source.SHA256 != fmt.Sprintf("%x", digest) {
		return fmt.Errorf("provenance checksum mismatch: got %q, want %x", source.SHA256, digest)
	}
	return nil
}

func validateOperations(document map[string]any) ([]string, error) {
	paths, ok := document["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		return nil, errors.New("schema has no paths")
	}
	seen := make(map[string]string)
	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %s is not an object", path)
		}
		for method, rawOperation := range item {
			if _, ok := httpMethods[strings.ToLower(method)]; !ok {
				continue
			}
			operation, ok := rawOperation.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s %s is not an object", strings.ToUpper(method), path)
			}
			id, _ := operation["operationId"].(string)
			if id == "" {
				return nil, fmt.Errorf("%s %s has no operationId", strings.ToUpper(method), path)
			}
			owner := strings.ToUpper(method) + " " + path
			if previous, exists := seen[id]; exists {
				return nil, fmt.Errorf("duplicate operationId %q on %s and %s", id, previous, owner)
			}
			seen[id] = owner
		}
	}
	operations := make([]string, 0, len(seen))
	for id, owner := range seen {
		operations = append(operations, owner+" "+id)
	}
	sort.Strings(operations)
	return operations, nil
}

func validateReferences(document map[string]any) error {
	components, _ := document["components"].(map[string]any)
	schemas, ok := components["schemas"].(map[string]any)
	if !ok || len(schemas) == 0 {
		return errors.New("schema has no component schemas")
	}
	return walk(document, func(ref string) error {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(ref, prefix) {
			return fmt.Errorf("unsupported external reference %q", ref)
		}
		name := strings.TrimPrefix(ref, prefix)
		if _, exists := schemas[name]; !exists {
			return fmt.Errorf("unresolved schema reference %q", ref)
		}
		return nil
	})
}

func walk(value any, visit func(string) error) error {
	switch node := value.(type) {
	case map[string]any:
		if ref, ok := node["$ref"].(string); ok {
			if err := visit(ref); err != nil {
				return err
			}
		}
		for _, child := range node {
			if err := walk(child, visit); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := walk(child, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

func compareOperations(expected, actual []string) error {
	sort.Strings(expected)
	sort.Strings(actual)
	missing, added := difference(expected, actual), difference(actual, expected)
	if len(missing) == 0 && len(added) == 0 {
		return nil
	}
	return fmt.Errorf("incompatible operation surface: missing=%v added=%v", missing, added)
}

func difference(left, right []string) []string {
	set := make(map[string]struct{}, len(right))
	for _, item := range right {
		set[item] = struct{}{}
	}
	var result []string
	for _, item := range left {
		if _, ok := set[item]; !ok {
			result = append(result, item)
		}
	}
	return result
}

func nonemptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for i := range left {
		diff |= left[i] ^ right[i]
	}
	return diff == 0
}

func isLowerHex(value string) bool {
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}
