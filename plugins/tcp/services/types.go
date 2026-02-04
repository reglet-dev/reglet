package services

// ConnectInput defines the input for TCP connection checks.
type ConnectInput struct {
	Host      string `json:"host" jsonschema:"required,description=Target hostname or IP"`
	Port      int    `json:"port" jsonschema:"required,description=Target port number"`
	TimeoutMs int    `json:"timeout_ms,omitempty" jsonschema:"default=5000,description=Connection timeout in milliseconds"`
	TLS       bool   `json:"tls,omitempty" jsonschema:"description=Use TLS connection"`
}

// ConnectOutput contains TCP connection details.
type ConnectOutput struct {
	Host           string `json:"host" jsonschema:"description=Connected host"`
	Port           int    `json:"port" jsonschema:"description=Connected port"`
	Connected      bool   `json:"connected" jsonschema:"description=Whether connection succeeded"`
	TLSVersion     string `json:"tls_version,omitempty" jsonschema:"description=Negotiated TLS version"`
	TLSCipherSuite string `json:"tls_cipher_suite,omitempty" jsonschema:"description=Negotiated cipher suite"`
	TLSExpiry      string `json:"tls_expiry,omitempty" jsonschema:"description=Certificate expiry date"`
	ResponseTimeMs int64  `json:"response_time_ms" jsonschema:"description=Connection time in milliseconds"`
	Error          string `json:"error,omitempty" jsonschema:"description=Error message if connection failed"`
}
