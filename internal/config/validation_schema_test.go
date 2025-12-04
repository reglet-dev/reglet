package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSchemaProvider is a mock implementation of PluginSchemaProvider for testing
type mockSchemaProvider struct {
	schemas map[string][]byte
}

func (m *mockSchemaProvider) GetPluginSchema(ctx context.Context, pluginName string) ([]byte, error) {
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

	profile := &Profile{
		Metadata: ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []Observation{
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

	profile := &Profile{
		Metadata: ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []Observation{
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

	profile := &Profile{
		Metadata: ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []Observation{
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

	profile := &Profile{
		Metadata: ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []Observation{
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
	profile := &Profile{
		Metadata: ProfileMetadata{
			Name: "", // Missing required name
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []Observation{
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

	profile := &Profile{
		Metadata: ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:   "test-1",
					Name: "Test Control 1",
					Observations: []Observation{
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
					Observations: []Observation{
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
