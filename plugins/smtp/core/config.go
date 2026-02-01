package core

// SMTPConfig defines the configuration for SMTP checks.
type SMTPConfig struct {
	Host      string `json:"host" jsonschema:"required,description=SMTP server host (hostname or IP)"`
	Port      int    `json:"port,omitempty" jsonschema:"default=25,description=SMTP server port"`
	TimeoutMs int    `json:"timeout_ms,omitempty" jsonschema:"default=5000,description=Connection timeout in milliseconds"`
	UseTLS    bool   `json:"use_tls,omitempty" jsonschema:"description=Use direct TLS/SSL connection (e.g. port 465)"`
	StartTLS  bool   `json:"use_starttls,omitempty" jsonschema:"description=Use STARTTLS to upgrade connection (e.g. port 587)"`
}
