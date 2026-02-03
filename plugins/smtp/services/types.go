package services

// ConnectInput defines the input for SMTP connection checks.
type ConnectInput struct {
	Host        string `json:"host" jsonschema:"required,description=SMTP server hostname"`
	Port        int    `json:"port,omitempty" jsonschema:"default=25,description=SMTP server port"`
	TimeoutMs   int    `json:"timeout_ms,omitempty" jsonschema:"default=10000,description=Connection timeout"`
	UseTLS      bool   `json:"use_tls,omitempty" jsonschema:"description=Use implicit TLS"`
	UseSTARTTLS bool   `json:"use_starttls,omitempty" jsonschema:"description=Upgrade to TLS via STARTTLS"`
}

// ConnectOutput contains SMTP connection details.
type ConnectOutput struct {
	Host         string   `json:"host" jsonschema:"description=Connected host"`
	Port         int      `json:"port" jsonschema:"description=Connected port"`
	Banner       string   `json:"banner" jsonschema:"description=SMTP banner message"`
	Extensions   []string `json:"extensions,omitempty" jsonschema:"description=Supported SMTP extensions"`
	TLSVersion   string   `json:"tls_version,omitempty" jsonschema:"description=TLS version if encrypted"`
	SupportsAuth bool     `json:"supports_auth" jsonschema:"description=Whether AUTH is supported"`
}
