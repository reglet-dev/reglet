package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// mockSchemaProvider is a mock implementation of PluginSchemaProvider for testing
type mockSchemaProvider struct {
	schemas   map[string][]byte
	callCount atomic.Int32 // Track how many times GetPluginSchema is called
}

func (m *mockSchemaProvider) GetPluginSchema(ctx context.Context, pluginName string) ([]byte, error) {
	m.callCount.Add(1)
	if schema, ok := m.schemas[pluginName]; ok {
		return schema, nil
	}
	return nil, nil // No schema available
}

// newMockSchemaProvider creates a mock schema provider with predefined schemas
func newMockSchemaProvider() *mockSchemaProvider {
	return &mockSchemaProvider{
		schemas: make(map[string][]byte),
	}
}

func (m *mockSchemaProvider) addSchema(pluginName string, schema map[string]interface{}) {
	schemaBytes, _ := json.Marshal(schema)
	m.schemas[pluginName] = schemaBytes
}

func (m *mockSchemaProvider) getCallCount() int32 {
	return m.callCount.Load()
}

func Test_ValidateWithSchemas_Success(t *testing.T) {
	// Setup mock provider with file plugin schema
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
			"exists": map[string]interface{}{
				"type": "boolean",
			},
		},
		"required": []string{"path"},
	})

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path":   "/etc/passwd",
								"exists": true,
							},
						},
					},
				},
			},
		},
	}

	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.NoError(t, err)
}

func Test_ValidateWithSchemas_TypeMismatch(t *testing.T) {
	// Setup mock provider with schema expecting integer
	provider := newMockSchemaProvider()
	provider.addSchema("http", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"port": map[string]interface{}{
				"type": "integer",
			},
			"url": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"url", "port"},
	})

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "http",
							Config: map[string]interface{}{
								"url":  "https://example.com",
								"port": "80", // String instead of integer!
							},
						},
					},
				},
			},
		},
	}

	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
	assert.Contains(t, err.Error(), "test-1")
	assert.Contains(t, err.Error(), "http")
}

func Test_ValidateWithSchemas_MissingRequiredField(t *testing.T) {
	// Setup mock provider with required fields
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"path"},
	})

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								// Missing required "path" field
								"exists": true,
							},
						},
					},
				},
			},
		},
	}

	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
	assert.Contains(t, err.Error(), "test-1")
}

func Test_ValidateWithSchemas_NoSchema(t *testing.T) {
	// Provider has no schema for the plugin
	provider := newMockSchemaProvider()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "unknown",
							Config: map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
		},
	}

	// Should succeed - no schema means no validation
	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.NoError(t, err)
}

func Test_ValidateWithSchemas_StructuralValidationFirst(t *testing.T) {
	// Provider with valid schema
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
	})

	// Profile with structural errors (should fail before schema validation)
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "", // Missing required name
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/passwd",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.Error(t, err)
	// Should fail on structural validation, not schema validation
	assert.Contains(t, err.Error(), "profile name is required")
}

func Test_ValidateWithSchemas_MultipleErrors(t *testing.T) {
	// Setup provider with schemas
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"path"},
	})
	provider.addSchema("http", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type": "string",
			},
			"port": map[string]interface{}{
				"type": "integer",
			},
		},
		"required": []string{"url"},
	})

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control 1",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								// Missing required "path" field
								"exists": true,
							},
						},
					},
				},
				{
					ID:   "test-2",
					Name: "Test Control 2",
					Observations: []entities.Observation{
						{
							Plugin: "http",
							Config: map[string]interface{}{
								"url":  "https://example.com",
								"port": "80", // Wrong type
							},
						},
					},
				},
			},
		},
	}

	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.Error(t, err)
	// Should report both errors
	assert.Contains(t, err.Error(), "test-1")
	assert.Contains(t, err.Error(), "test-2")
}

// SchemaCompiler tests

func Test_SchemaCompiler_CachesCompiledSchemas(t *testing.T) {
	// Setup mock provider with a schema
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"path"},
	})

	compiler := NewSchemaCompiler(provider)
	ctx := context.Background()

	// First call should fetch and compile
	schema1, err := compiler.GetCompiledSchema(ctx, "file")
	require.NoError(t, err)
	require.NotNil(t, schema1)
	assert.Equal(t, int32(1), provider.getCallCount())

	// Second call should return cached schema (no additional provider call)
	schema2, err := compiler.GetCompiledSchema(ctx, "file")
	require.NoError(t, err)
	require.NotNil(t, schema2)
	assert.Equal(t, int32(1), provider.getCallCount()) // Still 1!

	// Should return the same schema object
	assert.Equal(t, schema1, schema2)
}

func Test_SchemaCompiler_CachesDifferentPlugins(t *testing.T) {
	// Setup mock provider with multiple schemas
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
	})
	provider.addSchema("http", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{"type": "string"},
		},
	})

	compiler := NewSchemaCompiler(provider)
	ctx := context.Background()

	// Get file schema
	fileSchema, err := compiler.GetCompiledSchema(ctx, "file")
	require.NoError(t, err)
	require.NotNil(t, fileSchema)

	// Get http schema
	httpSchema, err := compiler.GetCompiledSchema(ctx, "http")
	require.NoError(t, err)
	require.NotNil(t, httpSchema)

	// Should have called provider twice (once per plugin)
	assert.Equal(t, int32(2), provider.getCallCount())

	// Get file schema again - should be cached
	fileSchema2, err := compiler.GetCompiledSchema(ctx, "file")
	require.NoError(t, err)
	assert.Equal(t, fileSchema, fileSchema2)
	assert.Equal(t, int32(2), provider.getCallCount()) // Still 2!
}

func Test_SchemaCompiler_CachesNilSchemas(t *testing.T) {
	// Setup mock provider with no schemas
	provider := newMockSchemaProvider()

	compiler := NewSchemaCompiler(provider)
	ctx := context.Background()

	// First call - plugin has no schema
	schema1, err := compiler.GetCompiledSchema(ctx, "unknown")
	require.NoError(t, err)
	assert.Nil(t, schema1)
	assert.Equal(t, int32(1), provider.getCallCount())

	// Second call - should return cached nil without calling provider again
	schema2, err := compiler.GetCompiledSchema(ctx, "unknown")
	require.NoError(t, err)
	assert.Nil(t, schema2)
	assert.Equal(t, int32(1), provider.getCallCount()) // Still 1!
}

func Test_SchemaCompiler_ThreadSafety(t *testing.T) {
	// Setup mock provider with a schema
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
	})

	compiler := NewSchemaCompiler(provider)
	ctx := context.Background()

	// Run 100 concurrent requests for the same schema
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			schema, err := compiler.GetCompiledSchema(ctx, "file")
			require.NoError(t, err)
			require.NotNil(t, schema)
		}()
	}

	wg.Wait()

	// Schema should have been compiled only once despite 100 concurrent calls
	// (There might be a few extra calls due to race between cache check and compile,
	// but it should be much less than 100)
	callCount := provider.getCallCount()
	assert.LessOrEqual(t, callCount, int32(5), "Schema should be cached, not compiled 100 times")
}

func Test_SchemaCompiler_ValidatesAgainstCompiledSchema(t *testing.T) {
	// Setup mock provider with a schema
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"path"},
	})

	compiler := NewSchemaCompiler(provider)
	ctx := context.Background()

	// Get compiled schema
	schema, err := compiler.GetCompiledSchema(ctx, "file")
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Valid config should pass
	validConfig := map[string]interface{}{
		"path": "/etc/passwd",
	}
	err = schema.Validate(validConfig)
	assert.NoError(t, err)

	// Invalid config should fail
	invalidConfig := map[string]interface{}{
		// Missing required "path" field
		"exists": true,
	}
	err = schema.Validate(invalidConfig)
	assert.Error(t, err)
}

func Test_ValidateWithSchemas_UsesCachedSchemas(t *testing.T) {
	// Setup mock provider with a schema
	provider := newMockSchemaProvider()
	provider.addSchema("file", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"path"},
	})

	// Profile with 50 observations using the same plugin
	controls := make([]entities.Control, 50)
	for i := 0; i < 50; i++ {
		controls[i] = entities.Control{
			ID:   fmt.Sprintf("test-%d", i),
			Name: "Test Control",
			Observations: []entities.Observation{
				{
					Plugin: "file",
					Config: map[string]interface{}{
						"path": "/etc/passwd",
					},
				},
			},
		}
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: controls,
		},
	}

	// Validate - should only call provider once despite 50 observations
	err := ValidateWithSchemas(context.Background(), profile, provider)
	require.NoError(t, err)

	// Schema should have been fetched only once
	assert.Equal(t, int32(1), provider.getCallCount())
}
