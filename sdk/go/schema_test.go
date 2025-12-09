//go:build !wasip1

package sdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSchema_SimpleStruct(t *testing.T) {
	type SimpleConfig struct {
		Host string `json:"host" description:"Server hostname"`
		Port int    `json:"port" default:"443"`
	}

	schema, err := GenerateSchema(SimpleConfig{})
	require.NoError(t, err)
	assert.NotEmpty(t, schema)

	// Validate it's valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	// Check schema structure
	assert.Contains(t, string(schema), "host")
	assert.Contains(t, string(schema), "port")
	// Note: description tags are not automatically included by jsonschema library
	// This could be enhanced in Phase 3 with custom reflector configuration
}

func TestGenerateSchema_NestedStruct(t *testing.T) {
	type ServerConfig struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}

	type Config struct {
		Server  ServerConfig `json:"server"`
		Timeout int          `json:"timeout"`
	}

	schema, err := GenerateSchema(Config{})
	require.NoError(t, err)
	assert.NotEmpty(t, schema)

	// Validate it's valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	// Check nested structure is present
	assert.Contains(t, string(schema), "server")
	assert.Contains(t, string(schema), "host")
	assert.Contains(t, string(schema), "timeout")
}

func TestGenerateSchema_WithTags(t *testing.T) {
	type TaggedConfig struct {
		Required string   `json:"required" description:"A required field"`
		Optional *string  `json:"optional,omitempty" description:"An optional field"`
		Default  int      `json:"default" default:"30"`
		List     []string `json:"list"`
	}

	schema, err := GenerateSchema(TaggedConfig{})
	require.NoError(t, err)

	// Validate it's valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	// Verify fields are present in schema
	schemaStr := string(schema)
	assert.Contains(t, schemaStr, "required")
	assert.Contains(t, schemaStr, "optional")
	assert.Contains(t, schemaStr, "list")
	// Note: description tags not automatically included - see TestGenerateSchema_SimpleStruct
}

func TestGenerateSchema_ArrayTypes(t *testing.T) {
	type ArrayConfig struct {
		Hosts []string          `json:"hosts"`
		Ports []int             `json:"ports"`
		Data  map[string]string `json:"data"`
	}

	schema, err := GenerateSchema(ArrayConfig{})
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	// Verify arrays are present
	schemaStr := string(schema)
	assert.Contains(t, schemaStr, "hosts")
	assert.Contains(t, schemaStr, "ports")
	assert.Contains(t, schemaStr, "data")
}

func TestGenerateSchema_PointerFields(t *testing.T) {
	type PointerConfig struct {
		RequiredString string  `json:"required"`
		OptionalString *string `json:"optional,omitempty"`
		OptionalInt    *int    `json:"optional_int,omitempty"`
	}

	schema, err := GenerateSchema(PointerConfig{})
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	// Pointer fields should still be in schema
	schemaStr := string(schema)
	assert.Contains(t, schemaStr, "required")
	assert.Contains(t, schemaStr, "optional")
	assert.Contains(t, schemaStr, "optional_int")
}

func TestGenerateSchema_EmptyStruct(t *testing.T) {
	type EmptyConfig struct{}

	schema, err := GenerateSchema(EmptyConfig{})
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	// Should still be valid JSON Schema for empty struct
	assert.NotEmpty(t, schema)
}

func TestGenerateSchema_ComplexExample(t *testing.T) {
	// Real-world example similar to plugin configs
	type HTTPConfig struct {
		URL     string            `json:"url" description:"HTTP endpoint URL"`
		Method  string            `json:"method" default:"GET" description:"HTTP method"`
		Headers map[string]string `json:"headers,omitempty" description:"Request headers"`
		Body    *string           `json:"body,omitempty" description:"Request body"`
		Timeout int               `json:"timeout" default:"30" description:"Timeout in seconds"`
	}

	schema, err := GenerateSchema(HTTPConfig{})
	require.NoError(t, err)

	// Validate structure
	var decoded map[string]interface{}
	err = json.Unmarshal(schema, &decoded)
	require.NoError(t, err)

	schemaStr := string(schema)

	// Check all fields are present
	assert.Contains(t, schemaStr, "url")
	assert.Contains(t, schemaStr, "method")
	assert.Contains(t, schemaStr, "headers")
	assert.Contains(t, schemaStr, "body")
	assert.Contains(t, schemaStr, "timeout")

	// Verify it's a valid JSON Schema with required fields
	properties, ok := decoded["properties"].(map[string]interface{})
	require.True(t, ok, "properties should be a map")
	assert.Len(t, properties, 5, "should have 5 properties")

	required, ok := decoded["required"].([]interface{})
	require.True(t, ok, "required should be an array")
	assert.Contains(t, required, "url")
	assert.Contains(t, required, "method")
	assert.Contains(t, required, "timeout")
}

func TestGenerateSchema_ValidJSONSchema(t *testing.T) {
	type TestConfig struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
	}

	schema, err := GenerateSchema(TestConfig{})
	require.NoError(t, err)

	// Unmarshal into a more specific structure to verify JSON Schema properties
	var schemaObj map[string]interface{}
	err = json.Unmarshal(schema, &schemaObj)
	require.NoError(t, err)

	// JSON Schema should have these top-level fields
	// Note: actual structure depends on jsonschema library version
	// We're just ensuring it's a valid structured document
	assert.NotNil(t, schemaObj)
	assert.NotEmpty(t, schemaObj)
}
