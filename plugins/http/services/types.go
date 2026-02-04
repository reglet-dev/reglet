package services

// RequestInput defines the input for HTTP requests.
type RequestInput struct {
	URL                string            `json:"url" jsonschema:"required,description=Target URL"`
	Method             string            `json:"method,omitempty" jsonschema:"enum=GET,enum=POST,enum=PUT,enum=DELETE,enum=HEAD,enum=OPTIONS,enum=PATCH,default=GET,description=HTTP method"`
	Headers            map[string]string `json:"headers,omitempty" jsonschema:"description=Request headers"`
	Body               string            `json:"body,omitempty" jsonschema:"description=Request body"`
	TimeoutSeconds     int               `json:"timeout_seconds,omitempty" jsonschema:"default=30,description=Request timeout"`
	FollowRedirects    bool              `json:"follow_redirects,omitempty" jsonschema:"default=true,description=Follow HTTP redirects"`
	InsecureSkipVerify bool              `json:"insecure_skip_verify,omitempty" jsonschema:"description=Skip TLS certificate verification"`
}

// RequestOutput contains HTTP request results.
type RequestOutput struct {
	URL                string              `json:"url" jsonschema:"description=Requested URL"`
	Method             string              `json:"method" jsonschema:"description=HTTP method used"`
	StatusCode         int                 `json:"status_code" jsonschema:"description=HTTP response status code"`
	Status             string              `json:"status" jsonschema:"description=HTTP status text"`
	Protocol           string              `json:"protocol" jsonschema:"description=HTTP protocol version"`
	Headers            map[string][]string `json:"headers" jsonschema:"description=Response headers"`
	Body               string              `json:"body,omitempty" jsonschema:"description=Response body (truncated if large)"`
	BodySize           int                 `json:"body_size" jsonschema:"description=Response body size in bytes"`
	TLSVersion         string              `json:"tls_version,omitempty" jsonschema:"description=TLS version if HTTPS"`
	TLSExpiry          string              `json:"tls_expiry,omitempty" jsonschema:"description=TLS certificate expiry date"`
	TLSDaysUntilExpiry int                 `json:"tls_days_until_expiry,omitempty" jsonschema:"description=Days until TLS certificate expires"`
	ResponseTimeMs     int64               `json:"response_time_ms" jsonschema:"description=Response time in milliseconds"`
}
