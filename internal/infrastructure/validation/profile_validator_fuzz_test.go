package validation

import (
	"context"
	"strings"
	"testing"

	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// FuzzPluginNameValidation fuzzes plugin name validation for path traversal and invalid chars
func FuzzPluginNameValidation(f *testing.F) {
	seeds := []string{
		"valid-plugin",
		"plugin_123",
		"../../etc/passwd",
		"plugin/with/slash",
		"plugin\\with\\backslash",
		strings.Repeat("a", 100),
		"",
		" ",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, name string) {
		// Should return error or nil, never panic
		_ = ValidatePluginName(name)
	})
}

// FuzzVersionValidation fuzzes version string validation
func FuzzVersionValidation(f *testing.F) {
	seeds := []string{
		"1.0.0",
		"0.0.1",
		"1.2.3-beta",
		"invalid",
		"1.2",
		"1.2.3.4",
		strings.Repeat("1.", 100),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, version string) {
		meta := entities.ProfileMetadata{
			Name:    "test",
			Version: version,
		}
		// Access private function validateMetadata
		_ = validateMetadata(meta)
	})
}

// FuzzSchemaValidation fuzzes the schema compiler and validator
func FuzzSchemaValidation(f *testing.F) {
	// Simple schema seed
	schemaJSON := `{
		"type": "object",
		"properties": {
			"foo": { "type": "string" }
		}
	}`

	seeds := []string{
		schemaJSON,
		`{"type": "string"}`,
		`{\"\$ref\": \"#/definitions/foo\"}`,
		strings.Repeat(`{"type":"object","properties":{"x":`, 50) + `{"type":"string"}` + strings.Repeat(`}}`, 50),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, schemaStr string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: %v", r)
			}
		}()

		// Mock provider that returns the fuzzed schema
		provider := &fuzzMockSchemaProvider{schema: []byte(schemaStr)}
		compiler := NewSchemaCompiler(provider)

		// Attempt to compile and validate against empty config
		ctx := context.Background()
		obs := entities.ObservationDefinition{
			Plugin: "fuzz-plugin",
			Config: map[string]interface{}{},
		}

		// This exercises GetCompiledSchema -> Compile -> Validate
		_ = validateObservationSchemaCompiled(ctx, obs, compiler)
	})
}

type fuzzMockSchemaProvider struct {
	schema []byte
}

func (m *fuzzMockSchemaProvider) GetPluginSchema(ctx context.Context, name string) ([]byte, error) {
	return m.schema, nil
}
