package core

// DNSConfig defines the configuration for DNS checks.
type DNSConfig struct {
	Hostname       string   `json:"hostname" jsonschema:"required,description=Hostname to resolve"`
	RecordType     string   `json:"record_type,omitempty" jsonschema:"enum=A,enum=AAAA,enum=MX,enum=TXT,enum=CNAME,enum=NS,default=A,description=DNS record type to query"`
	Nameserver     string   `json:"nameserver,omitempty" jsonschema:"description=Custom nameserver to use for DNS queries"`
	ExpectedValues []string `json:"expected_values,omitempty" jsonschema:"description=Expected DNS record values"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"default=10,minimum=1,maximum=60,description=Request timeout in seconds"`
}
