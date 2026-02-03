package core

// TCPConfig defines the configuration for TCP checks.
type TCPConfig struct {
	Host               string `json:"host" jsonschema:"required,description=Target host (hostname or IP)"`
	Port               int    `json:"port" jsonschema:"required,description=Target port (number)"`
	TimeoutMs          int    `json:"timeout_ms,omitempty" jsonschema:"default=5000,description=Connection timeout in milliseconds"`
	TLS                bool   `json:"tls,omitempty" jsonschema:"description=Use TLS/SSL connection"`
	ExpectedTLSVersion string `json:"expected_tls_version,omitempty" jsonschema:"enum=TLS 1.0,enum=TLS 1.1,enum=TLS 1.2,enum=TLS 1.3,description=Expected minimum TLS version"`
}
