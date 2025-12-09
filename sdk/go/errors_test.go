package sdk

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkError(t *testing.T) {
	baseErr := fmt.Errorf("connection refused")
	err := &NetworkError{
		Operation: "http_request",
		Target:    "api.example.com:443",
		Err:       baseErr,
	}

	assert.Equal(t, "network http_request failed for api.example.com:443: connection refused", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Equal(t, "http_request", netErr.Operation)
	assert.Equal(t, "api.example.com:443", netErr.Target)
}

func TestNetworkError_NoTarget(t *testing.T) {
	baseErr := fmt.Errorf("network unreachable")
	err := &NetworkError{
		Operation: "tcp_connect",
		Err:       baseErr,
	}

	assert.Equal(t, "network tcp_connect failed: network unreachable", err.Error())
}

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{
		Operation: "http_request",
		Duration:  5 * time.Second,
		Target:    "slow-api.example.com",
	}

	assert.Equal(t, "http_request timeout after 5s (target: slow-api.example.com)", err.Error())
	assert.True(t, err.Timeout())

	var timeoutErr *TimeoutError
	require.True(t, errors.As(err, &timeoutErr))
	assert.Equal(t, 5*time.Second, timeoutErr.Duration)
}

func TestTimeoutError_NoTarget(t *testing.T) {
	err := &TimeoutError{
		Operation: "dns_lookup",
		Duration:  2 * time.Second,
	}

	assert.Equal(t, "dns_lookup timeout after 2s", err.Error())
}

func TestCapabilityError(t *testing.T) {
	err := &CapabilityError{
		Required: "network:outbound",
		Pattern:  "api.example.com:443",
	}

	assert.Equal(t, "missing capability: network:outbound (pattern: api.example.com:443)", err.Error())

	var capErr *CapabilityError
	require.True(t, errors.As(err, &capErr))
	assert.Equal(t, "network:outbound", capErr.Required)
	assert.Equal(t, "api.example.com:443", capErr.Pattern)
}

func TestCapabilityError_NoPattern(t *testing.T) {
	err := &CapabilityError{
		Required: "exec",
	}

	assert.Equal(t, "missing capability: exec", err.Error())
}

func TestConfigError(t *testing.T) {
	baseErr := fmt.Errorf("invalid format")
	err := &ConfigError{
		Field: "hostname",
		Err:   baseErr,
	}

	assert.Equal(t, "config validation failed for field 'hostname': invalid format", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var confErr *ConfigError
	require.True(t, errors.As(err, &confErr))
	assert.Equal(t, "hostname", confErr.Field)
}

func TestConfigError_NoField(t *testing.T) {
	baseErr := fmt.Errorf("missing required fields")
	err := &ConfigError{
		Err: baseErr,
	}

	assert.Equal(t, "config validation failed: missing required fields", err.Error())
}

func TestExecError_DidNotRun(t *testing.T) {
	baseErr := fmt.Errorf("command not found")
	err := &ExecError{
		Command: "nonexistent-command",
		Err:     baseErr,
	}

	assert.Equal(t, "failed to execute 'nonexistent-command': command not found", err.Error())
	assert.True(t, errors.Is(err, baseErr))
}

func TestExecError_NonZeroExit(t *testing.T) {
	err := &ExecError{
		Command:  "grep",
		ExitCode: 1,
		Stderr:   "pattern not found",
	}

	assert.Equal(t, "command 'grep' exited with code 1: pattern not found", err.Error())

	var execErr *ExecError
	require.True(t, errors.As(err, &execErr))
	assert.Equal(t, 1, execErr.ExitCode)
	assert.Equal(t, "pattern not found", execErr.Stderr)
}

func TestExecError_NonZeroExitNoStderr(t *testing.T) {
	err := &ExecError{
		Command:  "false",
		ExitCode: 1,
	}

	assert.Equal(t, "command 'false' exited with code 1", err.Error())
}

func TestDNSError(t *testing.T) {
	baseErr := fmt.Errorf("no such host")
	err := &DNSError{
		Hostname:   "nonexistent.example.com",
		RecordType: "A",
		Nameserver: "8.8.8.8:53",
		Err:        baseErr,
	}

	assert.Equal(t, "dns lookup for nonexistent.example.com (A) via 8.8.8.8:53 failed: no such host", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var dnsErr *DNSError
	require.True(t, errors.As(err, &dnsErr))
	assert.Equal(t, "nonexistent.example.com", dnsErr.Hostname)
	assert.Equal(t, "A", dnsErr.RecordType)
}

func TestDNSError_NoNameserver(t *testing.T) {
	baseErr := fmt.Errorf("timeout")
	err := &DNSError{
		Hostname:   "example.com",
		RecordType: "AAAA",
		Err:        baseErr,
	}

	assert.Equal(t, "dns lookup for example.com (AAAA) failed: timeout", err.Error())
}

func TestDNSError_Timeout(t *testing.T) {
	timeoutErr := &TimeoutError{Operation: "dns", Duration: 5 * time.Second}
	err := &DNSError{
		Hostname:   "example.com",
		RecordType: "A",
		Err:        timeoutErr,
	}

	assert.True(t, err.Timeout())
}

func TestHTTPError(t *testing.T) {
	baseErr := fmt.Errorf("connection refused")
	err := &HTTPError{
		Method:     "GET",
		URL:        "https://api.example.com/status",
		StatusCode: 0,
		Err:        baseErr,
	}

	assert.Equal(t, "http GET https://api.example.com/status failed: connection refused", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var httpErr *HTTPError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, "GET", httpErr.Method)
	assert.Equal(t, "https://api.example.com/status", httpErr.URL)
}

func TestHTTPError_WithStatusCode(t *testing.T) {
	baseErr := fmt.Errorf("internal server error")
	err := &HTTPError{
		Method:     "POST",
		URL:        "https://api.example.com/data",
		StatusCode: 500,
		Err:        baseErr,
	}

	assert.Equal(t, "http POST https://api.example.com/data failed with status 500: internal server error", err.Error())
}

func TestHTTPError_Timeout(t *testing.T) {
	timeoutErr := &TimeoutError{Operation: "http", Duration: 10 * time.Second}
	err := &HTTPError{
		Method: "GET",
		URL:    "https://slow.example.com",
		Err:    timeoutErr,
	}

	assert.True(t, err.Timeout())
}

func TestTCPError(t *testing.T) {
	baseErr := fmt.Errorf("connection refused")
	err := &TCPError{
		Network: "tcp",
		Address: "example.com:443",
		Err:     baseErr,
	}

	assert.Equal(t, "tcp connect to example.com:443 (tcp) failed: connection refused", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var tcpErr *TCPError
	require.True(t, errors.As(err, &tcpErr))
	assert.Equal(t, "tcp", tcpErr.Network)
	assert.Equal(t, "example.com:443", tcpErr.Address)
}

func TestTCPError_Timeout(t *testing.T) {
	timeoutErr := &TimeoutError{Operation: "tcp_connect", Duration: 3 * time.Second}
	err := &TCPError{
		Network: "tcp",
		Address: "10.0.0.1:22",
		Err:     timeoutErr,
	}

	assert.True(t, err.Timeout())
}

func TestSchemaError(t *testing.T) {
	baseErr := fmt.Errorf("unsupported type")
	err := &SchemaError{
		Type: "MyCustomStruct",
		Err:  baseErr,
	}

	assert.Equal(t, "schema error for type MyCustomStruct: unsupported type", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var schemaErr *SchemaError
	require.True(t, errors.As(err, &schemaErr))
	assert.Equal(t, "MyCustomStruct", schemaErr.Type)
}

func TestSchemaError_NoType(t *testing.T) {
	baseErr := fmt.Errorf("invalid schema")
	err := &SchemaError{
		Err: baseErr,
	}

	assert.Equal(t, "schema error: invalid schema", err.Error())
}

func TestMemoryError(t *testing.T) {
	err := &MemoryError{
		Requested: 10 * 1024 * 1024,
		Current:   95 * 1024 * 1024,
		Limit:     100 * 1024 * 1024,
	}

	assert.Equal(t, "memory allocation failed: requested 10485760 bytes, current 99614720 bytes, limit 104857600 bytes", err.Error())

	var memErr *MemoryError
	require.True(t, errors.As(err, &memErr))
	assert.Equal(t, 10*1024*1024, memErr.Requested)
	assert.Equal(t, 100*1024*1024, memErr.Limit)
}

func TestWireFormatError(t *testing.T) {
	baseErr := fmt.Errorf("invalid json")
	err := &WireFormatError{
		Operation: "unmarshal",
		Type:      "DNSResponseWire",
		Err:       baseErr,
	}

	assert.Equal(t, "wire format unmarshal failed for DNSResponseWire: invalid json", err.Error())
	assert.True(t, errors.Is(err, baseErr))

	var wireErr *WireFormatError
	require.True(t, errors.As(err, &wireErr))
	assert.Equal(t, "unmarshal", wireErr.Operation)
	assert.Equal(t, "DNSResponseWire", wireErr.Type)
}

func TestErrorUnwrapping(t *testing.T) {
	baseErr := fmt.Errorf("base error")

	tests := []struct {
		name string
		err  error
	}{
		{"NetworkError", &NetworkError{Operation: "test", Err: baseErr}},
		{"ConfigError", &ConfigError{Field: "test", Err: baseErr}},
		{"ExecError", &ExecError{Command: "test", Err: baseErr}},
		{"DNSError", &DNSError{Hostname: "test", Err: baseErr}},
		{"HTTPError", &HTTPError{Method: "GET", URL: "test", Err: baseErr}},
		{"TCPError", &TCPError{Network: "tcp", Address: "test", Err: baseErr}},
		{"SchemaError", &SchemaError{Type: "test", Err: baseErr}},
		{"WireFormatError", &WireFormatError{Operation: "test", Type: "test", Err: baseErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, errors.Is(tt.err, baseErr), "errors.Is should find base error")
			unwrapped := errors.Unwrap(tt.err)
			assert.Equal(t, baseErr, unwrapped, "errors.Unwrap should return base error")
		})
	}
}
