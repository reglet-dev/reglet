package services

// ResolveInput defines the input for the Resolve operation.
type ResolveInput struct {
	// Hostname is the target to resolve (required)
	Hostname string `json:"hostname" jsonschema:"required,description=Target hostname to resolve"`

	// RecordType specifies which DNS record type to query
	RecordType string `json:"record_type,omitempty" jsonschema:"enum=A,enum=AAAA,enum=MX,enum=TXT,enum=CNAME,enum=NS,default=A,description=DNS record type to query"`

	// Nameserver optionally specifies a custom nameserver
	Nameserver string `json:"nameserver,omitempty" jsonschema:"description=Custom nameserver to use for resolution"`
}

// ResolveOutput defines the output for the Resolve operation.
type ResolveOutput struct {
	// Hostname is the queried hostname (echoed from input)
	Hostname string `json:"hostname" jsonschema:"description=Queried hostname"`

	// RecordType is the record type that was queried
	RecordType string `json:"record_type" jsonschema:"description=DNS record type queried"`

	// Records contains the resolved DNS records
	Records []string `json:"records" jsonschema:"description=Resolved DNS records"`

	// RecordCount is the number of resolved records
	RecordCount int `json:"record_count" jsonschema:"description=Number of resolved records"`

	// TTL is the time-to-live in seconds (if available)
	TTL int `json:"ttl,omitempty" jsonschema:"description=Record TTL in seconds"`

	// Nameserver is the nameserver used (if custom)
	Nameserver string `json:"nameserver,omitempty" jsonschema:"description=Nameserver used for resolution"`
}
