package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jrose/reglet/internal/wasm/hostfuncs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions for creating runtimes with proper capabilities

func newRuntimeWithNetworkCaps(ctx context.Context) (*Runtime, error) {
	caps := []hostfuncs.Capability{
		{Kind: "network", Pattern: "outbound:*"},
	}
	return NewRuntimeWithCapabilities(ctx, caps)
}

func newRuntimeWithFilesystemCaps(ctx context.Context) (*Runtime, error) {
	caps := []hostfuncs.Capability{
		{Kind: "fs", Pattern: "read:**"},
	}
	return NewRuntimeWithCapabilities(ctx, caps)
}

// TestLoadFilePlugin tests loading the actual file plugin WASM module
func TestLoadFilePlugin(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)
	require.NotEmpty(t, wasmBytes)

	// Create runtime
	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	// Load the plugin
	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, plugin)

	// Verify plugin is cached
	cachedPlugin, ok := runtime.GetPlugin("file")
	assert.True(t, ok)
	assert.Equal(t, plugin, cachedPlugin)

	// Verify plugin name
	assert.Equal(t, "file", plugin.Name())
}

// TestFilePlugin_Describe tests calling the describe function
// Uses Go 1.24+ //go:wasmexport for function exports
func TestFilePlugin_Describe(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Call describe and verify real plugin metadata
	info, err := plugin.Describe()
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify plugin metadata matches what the plugin exports
	assert.Equal(t, "file", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "File existence and content checks", info.Description)

	// Verify capabilities
	require.Len(t, info.Capabilities, 1)
	assert.Equal(t, "fs", info.Capabilities[0].Kind)
	assert.Equal(t, "read:**", info.Capabilities[0].Pattern)
}

// TestFilePlugin_Schema tests calling the schema function
func TestFilePlugin_Schema(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Call schema and verify we get valid JSON Schema
	schema, err := plugin.Schema()
	require.NoError(t, err)
	require.NotNil(t, schema)
	require.NotEmpty(t, schema.RawSchema)

	// Parse the JSON Schema to verify it's valid JSON
	var jsonSchema map[string]interface{}
	err = json.Unmarshal(schema.RawSchema, &jsonSchema)
	require.NoError(t, err)

	// Verify it's valid JSON Schema with expected fields
	require.NotEmpty(t, jsonSchema)
	assert.Equal(t, "object", jsonSchema["type"])

	// Verify properties exist
	props, ok := jsonSchema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "path")
	assert.Contains(t, props, "mode")
}

// TestFilePlugin_Observe_FileExists tests checking if a file exists
func TestFilePlugin_Observe_FileExists(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "reglet-test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := newRuntimeWithFilesystemCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Test file exists check
	config := Config{
		Values: map[string]string{
			"path": tmpFile.Name(),
			"mode": "exists",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify the file was found
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)
	assert.Equal(t, tmpFile.Name(), result.Evidence.Data["path"])
	assert.Equal(t, "exists", result.Evidence.Data["mode"])
}

// TestFilePlugin_Observe_FileNotFound tests checking a non-existent file
func TestFilePlugin_Observe_FileNotFound(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := newRuntimeWithFilesystemCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Test non-existent file
	config := Config{
		Values: map[string]string{
			"path": "/tmp/reglet-nonexistent-file-12345.txt",
			"mode": "exists",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error, "file not found should return evidence with status=false, not an error")
	require.NotNil(t, result.Evidence)

	// Verify the file was not found
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.False(t, status, "status should be false for non-existent file")
	assert.Contains(t, result.Evidence.Data, "error", "should include error message in evidence")

	// Verify the error message makes sense
	errMsg, ok := result.Evidence.Data["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errMsg, "such file", "error message should indicate file not found")
}

// TestFilePlugin_Observe_ReadContent tests reading file content
func TestFilePlugin_Observe_ReadContent(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Create a temporary test file with known content
	tmpFile, err := os.CreateTemp("", "reglet-test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	testContent := "Hello from Reglet!"
	tmpFile.WriteString(testContent)
	tmpFile.Close()

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := newRuntimeWithFilesystemCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Test content reading
	config := Config{
		Values: map[string]string{
			"path": tmpFile.Name(),
			"mode": "content",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify the content was read
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	content, ok := result.Evidence.Data["content"].(string)
	require.True(t, ok)
	assert.Equal(t, testContent, content)

	// Verify size
	size, ok := result.Evidence.Data["size"].(float64) // JSON numbers are float64
	require.True(t, ok)
	assert.Equal(t, float64(len(testContent)), size)
}
// DNS Plugin Tests

// TestDNSPlugin_Describe tests DNS plugin metadata
func TestDNSPlugin_Describe(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "dns", "dns.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("dns.wasm not built - run 'make -C plugins/dns build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("dns", wasmBytes)
	require.NoError(t, err)

	info, err := plugin.Describe()
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "dns", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "DNS resolution and record validation", info.Description)

	require.Len(t, info.Capabilities, 1)
	assert.Equal(t, "network", info.Capabilities[0].Kind)
	assert.Equal(t, "outbound:53", info.Capabilities[0].Pattern)
}

// TestDNSPlugin_Schema tests DNS plugin configuration schema
func TestDNSPlugin_Schema(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "dns", "dns.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("dns.wasm not built - run 'make -C plugins/dns build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("dns", wasmBytes)
	require.NoError(t, err)

	schema, err := plugin.Schema()
	require.NoError(t, err)
	require.NotNil(t, schema)
	require.NotEmpty(t, schema.RawSchema)

	// Parse the JSON Schema to verify it's valid JSON
	var schemaData map[string]interface{}
	err = json.Unmarshal(schema.RawSchema, &schemaData)
	require.NoError(t, err)

	// Verify schema structure
	assert.Equal(t, "object", schemaData["type"])
	properties, ok := schemaData["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "hostname")
	assert.Contains(t, properties, "record_type")
	assert.Contains(t, properties, "nameserver")

	required, ok := schemaData["required"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, required, "hostname")
}

// TestDNSPlugin_Observe_A_Record tests A record lookup
func TestDNSPlugin_Observe_A_Record(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "dns", "dns.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("dns.wasm not built - run 'make -C plugins/dns build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("dns", wasmBytes)
	require.NoError(t, err)

	// Test A record lookup for example.com
	config := Config{
		Values: map[string]string{
			"hostname":    "example.com",
			"record_type": "A",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify success
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	// Verify records returned
	records, ok := result.Evidence.Data["records"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, records, "should return at least one A record")

	// Verify record count
	recordCount, ok := result.Evidence.Data["record_count"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(len(records)), recordCount)

	// Verify query time present
	_, ok = result.Evidence.Data["query_time_ms"].(float64)
	assert.True(t, ok, "should include query time")
}

// TestDNSPlugin_Observe_MX_Record tests MX record lookup
func TestDNSPlugin_Observe_MX_Record(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "dns", "dns.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("dns.wasm not built - run 'make -C plugins/dns build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("dns", wasmBytes)
	require.NoError(t, err)

	// Test MX record lookup for gmail.com (known to have MX records)
	config := Config{
		Values: map[string]string{
			"hostname":    "gmail.com",
			"record_type": "MX",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)

	// MX lookup should succeed
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	records, ok := result.Evidence.Data["records"].([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, records, "gmail.com should have MX records")
}

// TestDNSPlugin_Observe_InvalidHostname tests error handling
func TestDNSPlugin_Observe_InvalidHostname(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "dns", "dns.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("dns.wasm not built - run 'make -C plugins/dns build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("dns", wasmBytes)
	require.NoError(t, err)

	// Test with non-existent hostname
	config := Config{
		Values: map[string]string{
			"hostname": "this-definitely-does-not-exist-12345.invalid",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error, "DNS lookup failure should return evidence with status=false, not an error")
	require.NotNil(t, result.Evidence)

	// Verify DNS lookup failed
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.False(t, status, "status should be false for failed DNS lookup")
	assert.Contains(t, result.Evidence.Data, "error", "should include error message in evidence")
}

// TestDNSPlugin_Observe_MissingHostname tests config validation
func TestDNSPlugin_Observe_MissingHostname(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "dns", "dns.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("dns.wasm not built - run 'make -C plugins/dns build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("dns", wasmBytes)
	require.NoError(t, err)

	// Test with missing hostname
	config := Config{
		Values: map[string]string{},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error, "validation errors should return evidence with status=false, not an error")
	require.NotNil(t, result.Evidence)

	// Should return status=false with error message in evidence
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.False(t, status, "status should be false for validation error")

	errMsg, ok := result.Evidence.Data["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errMsg, "missing required field: hostname")
}

// ============================================================================
// HTTP Plugin Tests
// ============================================================================

// TestHTTPPlugin_Describe tests HTTP plugin describe function
func TestHTTPPlugin_Describe(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "http", "http.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("http.wasm not built - run 'make -C plugins/http build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("http", wasmBytes)
	require.NoError(t, err)

	info, err := plugin.Describe()
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "http", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "HTTP")
}

// TestHTTPPlugin_Schema tests HTTP plugin schema function
func TestHTTPPlugin_Schema(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "http", "http.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("http.wasm not built - run 'make -C plugins/http build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("http", wasmBytes)
	require.NoError(t, err)

	schema, err := plugin.Schema()
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Parse schema JSON
	var schemaData map[string]interface{}
	err = json.Unmarshal([]byte(schema.RawSchema), &schemaData)
	require.NoError(t, err)

	// Verify schema has expected properties
	properties, ok := schemaData["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "url")
	assert.Contains(t, properties, "method")
}

// TestHTTPPlugin_Observe_GET tests HTTP GET request
func TestHTTPPlugin_Observe_GET(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "http", "http.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("http.wasm not built - run 'make -C plugins/http build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("http", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]string{
			"url":    "https://example.com",
			"method": "GET",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify response
	statusCode, ok := result.Evidence.Data["status_code"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(200), statusCode)

	body, ok := result.Evidence.Data["body"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, body)
}

// ============================================================================
// TCP Plugin Tests
// ============================================================================

// TestTCPPlugin_Describe tests TCP plugin describe function
func TestTCPPlugin_Describe(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "tcp", "tcp.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("tcp.wasm not built - run 'make -C plugins/tcp build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("tcp", wasmBytes)
	require.NoError(t, err)

	info, err := plugin.Describe()
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "tcp", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "TCP")
}

// TestTCPPlugin_Schema tests TCP plugin schema function
func TestTCPPlugin_Schema(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "tcp", "tcp.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("tcp.wasm not built - run 'make -C plugins/tcp build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("tcp", wasmBytes)
	require.NoError(t, err)

	schema, err := plugin.Schema()
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Parse schema JSON
	var schemaData map[string]interface{}
	err = json.Unmarshal([]byte(schema.RawSchema), &schemaData)
	require.NoError(t, err)

	// Verify schema has expected properties
	properties, ok := schemaData["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, properties, "host")
	assert.Contains(t, properties, "port")
	assert.Contains(t, properties, "tls")
}

// TestTCPPlugin_Observe_PlainTCP tests plain TCP connection
func TestTCPPlugin_Observe_PlainTCP(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "tcp", "tcp.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("tcp.wasm not built - run 'make -C plugins/tcp build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("tcp", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]string{
			"host": "example.com",
			"port": "80",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify connection succeeded
	connected, ok := result.Evidence.Data["connected"].(bool)
	require.True(t, ok)
	assert.True(t, connected)

	address, ok := result.Evidence.Data["address"].(string)
	require.True(t, ok)
	assert.Equal(t, "example.com:80", address)
}

// TestTCPPlugin_Observe_TLS tests TLS connection
func TestTCPPlugin_Observe_TLS(t *testing.T) {
	wasmPath := filepath.Join("..", "..", "plugins", "tcp", "tcp.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("tcp.wasm not built - run 'make -C plugins/tcp build' first")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	ctx := context.Background()
	runtime, err := newRuntimeWithNetworkCaps(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("tcp", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]string{
			"host": "example.com",
			"port": "443",
			"tls":  "true",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify TLS connection succeeded
	connected, ok := result.Evidence.Data["connected"].(bool)
	require.True(t, ok)
	assert.True(t, connected)

	tlsVersion, ok := result.Evidence.Data["tls_version"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, tlsVersion)
	assert.Contains(t, tlsVersion, "TLS")

	cipherSuite, ok := result.Evidence.Data["tls_cipher_suite"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, cipherSuite)
}
