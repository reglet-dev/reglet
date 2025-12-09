package wasm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"
)

// Global cache for WASM bytes to avoid repeated disk I/O
var (
	wasmCache = make(map[string][]byte)
	cacheMu   sync.Mutex
)

// getKeys returns the keys of a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getWasmBytes returns the bytes of the requested plugin WASM file.
// It caches the result for future calls.
func getWasmBytes(t *testing.T, pluginName string) []byte {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if bytes, ok := wasmCache[pluginName]; ok {
		return bytes
	}

	wasmPath := filepath.Join("..", "..", "plugins", pluginName, pluginName+".wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skipf("%s.wasm not built - run 'make -C plugins/%s build' first", pluginName, pluginName)
	}

	bytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)
	require.NotEmpty(t, bytes)

	wasmCache[pluginName] = bytes
	return bytes
}

// TestLoadFilePlugin tests loading the actual file plugin WASM module
func TestLoadFilePlugin(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Load the plugin
	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
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
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Call describe and verify real plugin metadata
	info, err := plugin.Describe(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify plugin metadata matches what the plugin exports
	assert.Equal(t, "file", info.Name)
	assert.Equal(t, "1.1.0", info.Version)
	assert.Equal(t, "File existence, content, and hash checks", info.Description)

	// Verify capabilities
	require.Len(t, info.Capabilities, 1)
	assert.Equal(t, "fs", info.Capabilities[0].Kind)
	assert.Equal(t, "read:**", info.Capabilities[0].Pattern)
}

// TestFilePlugin_Schema tests calling the schema function
func TestFilePlugin_Schema(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Call schema and verify we get valid JSON Schema
	schema, err := plugin.Schema(ctx)
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
	assert.Contains(t, props, "read_content")
	assert.Contains(t, props, "hash")
}

// TestFilePlugin_Observe_FileExists tests checking if a file exists
func TestFilePlugin_Observe_FileExists(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// Create a temporary test file
	// Note: t.TempDir is better for parallelism, but os.CreateTemp is fine if handled correctly
	tmpFile, err := os.CreateTemp(".", "reglet-test-*.txt")
	require.NoError(t, err)
	// We cannot defer remove here if we use t.Parallel() and share filesystem resources in a way that conflicts
	// But here each test creates its own file, so it's fine.
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Test file exists check
	config := Config{
		Values: map[string]interface{}{
			"path": tmpFile.Name(),
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify the file was found
	t.Logf("Evidence.Data keys: %v", getKeys(result.Evidence.Data))
	t.Logf("Full Evidence.Data: %+v", result.Evidence.Data)
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)
	assert.Equal(t, tmpFile.Name(), result.Evidence.Data["path"])

	// Verify ownership
	uid, ok := result.Evidence.Data["uid"].(float64)
	require.True(t, ok, "uid should be present")
	assert.GreaterOrEqual(t, uid, float64(0))

	gid, ok := result.Evidence.Data["gid"].(float64)
	require.True(t, ok, "gid should be present")
	assert.GreaterOrEqual(t, gid, float64(0))
}

// TestFilePlugin_Observe_Symlink tests checking a symlink
func TestFilePlugin_Observe_Symlink(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// Create a temporary test file and symlink in current directory
	// to ensure WASM runtime access (similar to other tests)
	targetFile, err := os.CreateTemp(".", "reglet-target-*.txt")
	require.NoError(t, err)
	targetName := targetFile.Name()
	targetFile.Close()
	defer os.Remove(targetName)

	err = os.WriteFile(targetName, []byte("content"), 0644)
	require.NoError(t, err)

	linkName := targetName + ".link"
	err = os.Symlink(targetName, linkName)
	require.NoError(t, err)
	defer os.Remove(linkName)

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Test symlink check
	config := Config{
		Values: map[string]interface{}{
			"path": linkName,
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)

	// Verify symlink details
	t.Logf("Evidence.Data: %+v", result.Evidence.Data)
	isSymlink, ok := result.Evidence.Data["is_symlink"].(bool)
	require.True(t, ok, "is_symlink should be present")
	assert.True(t, isSymlink, "should be identified as symlink")

	target, ok := result.Evidence.Data["symlink_target"].(string)
	require.True(t, ok, "symlink_target should be present")
	assert.Equal(t, targetName, target)
}

// TestFilePlugin_Observe_FileNotFound tests checking a non-existent file
func TestFilePlugin_Observe_FileNotFound(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Test non-existent file
	config := Config{
		Values: map[string]interface{}{
			"path": "/tmp/reglet-nonexistent-file-12345.txt",
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Evidence)

	// Verify the check succeeded (status=true) but file does not exist (exists=false)
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status, "status should be true (observation successful)")

	exists, ok := result.Evidence.Data["exists"].(bool)
	require.True(t, ok)
	assert.False(t, exists, "exists should be false for missing file")
}

// TestFilePlugin_Observe_ReadContent tests reading file content
func TestFilePlugin_Observe_ReadContent(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// Create a temporary test file with known content
	tmpFile, err := os.CreateTemp(".", "reglet-test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	testContent := "Hello from Reglet!"
	tmpFile.WriteString(testContent)
	tmpFile.Close()

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Test content reading
	config := Config{
		Values: map[string]interface{}{
			"path":         tmpFile.Name(),
			"read_content": true,
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify the content was read
	t.Logf("Evidence.Data keys: %v", getKeys(result.Evidence.Data))
	t.Logf("Full Evidence.Data: %+v", result.Evidence.Data)

	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	// Verify base64 content
	contentB64, ok := result.Evidence.Data["content_b64"].(string)
	require.True(t, ok, "content_b64 field should be present")

	// Verify encoding field
	encoding, ok := result.Evidence.Data["encoding"].(string)
	require.True(t, ok)
	assert.Equal(t, "base64", encoding)

	// Decode and verify content
	decoded, err := base64.StdEncoding.DecodeString(contentB64)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(decoded))

	// Verify size
	size, ok := result.Evidence.Data["size"].(float64) // JSON numbers are float64
	require.True(t, ok)
	assert.Equal(t, float64(len(testContent)), size)
}

// TestFilePlugin_Observe_BinaryContent tests reading binary file content
func TestFilePlugin_Observe_BinaryContent(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "file")

	// Create a temporary test file with binary content (including null bytes)
	binaryData := []byte{0x00, 0xFF, 0xFE, 0xAB, 0xCD, 0x00, 0x01, 0x02, 0x7F, 0x80, 0x81}
	tmpFile, err := os.CreateTemp(".", "reglet-binary-test-*.bin")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(binaryData)
	require.NoError(t, err)
	tmpFile.Close()

	// File plugin needs filesystem capabilities
	caps := map[string][]hostfuncs.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:**"},
		},
	}

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Test binary content reading
	config := Config{
		Values: map[string]interface{}{
			"path":         tmpFile.Name(),
			"read_content": true,
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify status
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	// Verify base64 content field exists
	contentB64, ok := result.Evidence.Data["content_b64"].(string)
	require.True(t, ok, "content_b64 field should be present")

	// Verify encoding field
	encoding, ok := result.Evidence.Data["encoding"].(string)
	require.True(t, ok)
	assert.Equal(t, "base64", encoding)

	// Decode and verify binary content
	decoded, err := base64.StdEncoding.DecodeString(contentB64)
	require.NoError(t, err, "base64 decoding should succeed")
	assert.Equal(t, binaryData, decoded, "decoded content should match original binary data")

	// Verify size
	size, ok := result.Evidence.Data["size"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(len(binaryData)), size)
}

// DNS Plugin Tests

// TestDNSPlugin_Describe tests DNS plugin metadata
func TestDNSPlugin_Describe(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "dns")

	// DNS plugin needs network capabilities for port 53
	caps := map[string][]hostfuncs.Capability{
		"dns": {
			{Kind: "network", Pattern: "outbound:53"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "dns", wasmBytes)
	require.NoError(t, err)

	info, err := plugin.Describe(ctx)
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
	t.Parallel()
	wasmBytes := getWasmBytes(t, "dns")

	// DNS plugin needs network capabilities for port 53
	caps := map[string][]hostfuncs.Capability{
		"dns": {
			{Kind: "network", Pattern: "outbound:53"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "dns", wasmBytes)
	require.NoError(t, err)

	schema, err := plugin.Schema(ctx)
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
	t.Parallel()
	wasmBytes := getWasmBytes(t, "dns")

	// DNS plugin needs network capabilities for port 53
	caps := map[string][]hostfuncs.Capability{
		"dns": {
			{Kind: "network", Pattern: "outbound:53"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "dns", wasmBytes)
	require.NoError(t, err)

	// Test A record lookup for example.com
	config := Config{
		Values: map[string]interface{}{
			"hostname":    "example.com",
			"record_type": "A",
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Evidence)
	require.Nil(t, result.Error) // The Go error should be nil, status in evidence.

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

	// Ensure MX records are empty
	_, ok = result.Evidence.Data["mx_records"]
	assert.False(t, ok, "mx_records should not be present for A record lookup")

	// Ensure error flags are not set
	assert.False(t, result.Evidence.Data["is_not_found"].(bool), "is_not_found should be false")
	assert.False(t, result.Evidence.Data["is_timeout"].(bool), "is_timeout should be false")
}

// TestDNSPlugin_Observe_MX_Record tests MX record lookup
func TestDNSPlugin_Observe_MX_Record(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "dns")

	// DNS plugin needs network capabilities for port 53
	caps := map[string][]hostfuncs.Capability{
		"dns": {
			{Kind: "network", Pattern: "outbound:53"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "dns", wasmBytes)
	require.NoError(t, err)

	// Test MX record lookup for gmail.com (known to have MX records)
	config := Config{
		Values: map[string]interface{}{
			"hostname":    "gmail.com",
			"record_type": "MX",
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Evidence)
	require.Nil(t, result.Error) // The Go error should be nil, status in evidence.

	// MX lookup should succeed
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	// Verify structured MX records returned
	mxRecords, ok := result.Evidence.Data["mx_records"].([]interface{})
	require.True(t, ok, "mx_records should be present")
	assert.NotEmpty(t, mxRecords, "gmail.com should have MX records")

	// Check structure of first record
	firstMX, ok := mxRecords[0].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, firstMX, "host")
	assert.Contains(t, firstMX, "pref")
	assert.IsType(t, "", firstMX["host"])
	assert.IsType(t, float64(0), firstMX["pref"]) // JSON numbers are float64

	// Verify record count
	recordCount, ok := result.Evidence.Data["record_count"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(len(mxRecords)), recordCount)

	// Ensure generic records are empty
	_, ok = result.Evidence.Data["records"]
	assert.False(t, ok, "records should not be present for MX record lookup (structured MX is used)")

	// Ensure error flags are not set
	assert.False(t, result.Evidence.Data["is_not_found"].(bool), "is_not_found should be false")
	assert.False(t, result.Evidence.Data["is_timeout"].(bool), "is_timeout should be false")
}

// TestDNSPlugin_Observe_InvalidHostname tests error handling
func TestDNSPlugin_Observe_InvalidHostname(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "dns")

	// DNS plugin needs network capabilities for port 53
	caps := map[string][]hostfuncs.Capability{
		"dns": {
			{Kind: "network", Pattern: "outbound:53"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "dns", wasmBytes)
	require.NoError(t, err)

	// Test with non-existent hostname
	config := Config{
		Values: map[string]interface{}{
			"hostname":    "this-definitely-does-not-exist-12345.invalid",
			"record_type": "A",
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Evidence)
	require.Nil(t, result.Error, "Observe should return a nil Go error, expected nil for successful observation")

	// Verify DNS lookup failed
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.False(t, status, "status should be false for failed DNS lookup")

	// Verify error flags from Evidence.Data (populated by plugin)
	isNotFound, ok := result.Evidence.Data["is_not_found"].(bool)
	require.True(t, ok, "is_not_found should be present in Evidence.Data")
	assert.True(t, isNotFound, "is_not_found in Evidence.Data should be true for non-existent host")
	isTimeout, ok := result.Evidence.Data["is_timeout"].(bool)
	require.True(t, ok, "is_timeout should be present in Evidence.Data")
	assert.False(t, isTimeout, "is_timeout in Evidence.Data should be false")

	// Verify error message from Evidence.Data
	errMsgData, ok := result.Evidence.Data["error_message"].(string)
	require.True(t, ok, "error_message should be present in Evidence.Data")
	assert.Contains(t, errMsgData, "no such host", "error message in Evidence.Data should indicate no such host")

	// Records should be empty
	_, recordsPresent := result.Evidence.Data["records"]
	assert.False(t, recordsPresent, "records key should not be present for failed lookup")
	_, mxRecordsPresent := result.Evidence.Data["mx_records"]
	assert.False(t, mxRecordsPresent, "mx_records key should not be present")
}

// TestDNSPlugin_Observe_MissingHostname tests config validation
func TestDNSPlugin_Observe_MissingHostname(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "dns")

	// DNS plugin needs network capabilities for port 53
	caps := map[string][]hostfuncs.Capability{
		"dns": {
			{Kind: "network", Pattern: "outbound:53"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "dns", wasmBytes)
	require.NoError(t, err)

	// Test with missing hostname
	config := Config{
		Values: map[string]interface{}{},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Evidence)
	require.Nil(t, result.Error, "Observe should return nil error for config validation error")

	// Should return status=false for validation error
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.False(t, status, "status should be false for validation error")

	// Verify that Evidence.Data["error"] contains the structured error (map)
	errData, ok := result.Evidence.Data["error"].(map[string]interface{})
	require.True(t, ok, "Error details should be in Evidence.Data['error'] map")

	errMsg, ok := errData["message"].(string)
	require.True(t, ok, "Error message should be in error map")
	assert.Contains(t, errMsg, "config validation failed: Key: 'DNSConfig.Hostname'", "error message should indicate missing Hostname")
}

// TestHTTPPlugin_Describe tests HTTP plugin describe function
func TestHTTPPlugin_Describe(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "http")

	// HTTP plugin needs network capabilities for ports 80,443
	caps := map[string][]hostfuncs.Capability{
		"http": {
			{Kind: "network", Pattern: "outbound:80,443"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "http", wasmBytes)
	require.NoError(t, err)

	info, err := plugin.Describe(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "http", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "HTTP")
}

// TestHTTPPlugin_Schema tests HTTP plugin schema function
func TestHTTPPlugin_Schema(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "http")

	// HTTP plugin needs network capabilities for ports 80,443
	caps := map[string][]hostfuncs.Capability{
		"http": {
			{Kind: "network", Pattern: "outbound:80,443"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "http", wasmBytes)
	require.NoError(t, err)

	schema, err := plugin.Schema(ctx)
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
	t.Parallel() // Restore t.Parallel()
	wasmBytes := getWasmBytes(t, "http")

	// HTTP plugin needs network capabilities for ports 80,443
	caps := map[string][]hostfuncs.Capability{
		"http": {
			{Kind: "network", Pattern: "outbound:80,443"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "http", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]interface{}{
			"url":                 "https://example.com",
			"method":              "GET",
			"body_preview_length": -1, // Get full body for testing
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify response
	statusCode, ok := result.Evidence.Data["status_code"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(200), statusCode)

	body, ok := result.Evidence.Data["body"].(string)
	require.True(t, ok, "Expected 'body' field in response data. Got: %+v", result.Evidence.Data)
	assert.NotEmpty(t, body)

	// Verify new fields (response_time_ms, protocol, headers)
	_, ok = result.Evidence.Data["response_time_ms"].(float64)
	assert.True(t, ok, "response_time_ms should be present")

	protocol, ok := result.Evidence.Data["protocol"].(string)
	assert.True(t, ok, "protocol should be present")
	assert.NotEmpty(t, protocol)

	// Headers is map[string][]string, but unmarshaled as map[string]interface{}
	headers, ok := result.Evidence.Data["headers"].(map[string]interface{})
	assert.True(t, ok, "headers should be present")
	assert.NotEmpty(t, headers)
}

// ============================================================================
// TCP Plugin Tests
// ============================================================================

func TestTCPPlugin_Describe(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "tcp")

	// TCP plugin needs network capabilities for outbound connections
	caps := map[string][]hostfuncs.Capability{
		"tcp": {
			{Kind: "network", Pattern: "outbound:*"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "tcp", wasmBytes)
	require.NoError(t, err)

	info, err := plugin.Describe(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "tcp", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "TCP")
}

// TestTCPPlugin_Schema tests TCP plugin schema function
func TestTCPPlugin_Schema(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "tcp")

	// TCP plugin needs network capabilities for outbound connections
	caps := map[string][]hostfuncs.Capability{
		"tcp": {
			{Kind: "network", Pattern: "outbound:*"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "tcp", wasmBytes)
	require.NoError(t, err)

	schema, err := plugin.Schema(ctx)
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

func TestTCPPlugin_Observe_PlainTCP(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "tcp")

	// TCP plugin needs network capabilities for outbound connections
	caps := map[string][]hostfuncs.Capability{
		"tcp": {
			{Kind: "network", Pattern: "outbound:*"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "tcp", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]interface{}{
			"host": "example.com",
			"port": "80",
		},
	}

	result, err := plugin.Observe(ctx, config)
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

func TestTCPPlugin_Observe_TLS(t *testing.T) {
	t.Parallel()
	wasmBytes := getWasmBytes(t, "tcp")

	// TCP plugin needs network capabilities for outbound connections
	caps := map[string][]hostfuncs.Capability{
		"tcp": {
			{Kind: "network", Pattern: "outbound:*"},
		},
	}

	ctx := context.Background()
	runtime, err := NewRuntimeWithCapabilities(ctx, caps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "tcp", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]interface{}{
			"host": "example.com",
			"port": "443",
			"tls":  true,
		},
	}

	result, err := plugin.Observe(ctx, config)
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

	// Verify certificate details
	certNotAfter, ok := result.Evidence.Data["tls_cert_not_after"].(string)
	require.True(t, ok, "tls_cert_not_after should be present")
	assert.NotEmpty(t, certNotAfter)

	daysRemaining, ok := result.Evidence.Data["tls_cert_days_remaining"].(float64)
	require.True(t, ok, "tls_cert_days_remaining should be present")
	assert.True(t, daysRemaining > 0, "days remaining should be positive for a valid cert")
}
