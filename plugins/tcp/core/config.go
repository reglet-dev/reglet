package core

// TCPConfig defines the configuration for TCP checks.
type TCPConfig struct {
	// Port is defined as int here. Users providing string must ensure it parses.
	// We could use interface{} but strict schema is better for SDK.
	// For backward compatibility, the plugin.go Check logic might need to handle adaptation if we keep it loose there?
	// But core config is usually strict.
	// Let's use int for Port as recommended.
	Host               string `json:"host" jsonschema:"required,description=Target host (hostname or IP)"`
	Port               int    `json:"port" jsonschema:"required,description=Target port (number)"`
	TimeoutMs          int    `json:"timeout_ms,omitempty" jsonschema:"default=5000,description=Connection timeout in milliseconds"`
	TLS                bool   `json:"tls,omitempty" jsonschema:"description=Use TLS/SSL connection"`
	ExpectedTLSVersion string `json:"expected_tls_version,omitempty" jsonschema:"enum=TLS 1.0,enum=TLS 1.1,enum=TLS 1.2,enum=TLS 1.3,description=Expected minimum TLS version"`
}
