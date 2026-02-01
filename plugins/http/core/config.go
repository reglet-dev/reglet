package core

// HTTPConfig defines the configuration for HTTP checks.
type HTTPConfig struct {
	URL                  string            `json:"url" jsonschema:"required,description=URL to request"`
	Method               string            `json:"method,omitempty" jsonschema:"enum=GET,enum=POST,enum=PUT,enum=DELETE,enum=HEAD,enum=OPTIONS,enum=PATCH,default=GET,description=HTTP method"`
	Headers              map[string]string `json:"headers,omitempty" jsonschema:"description=HTTP headers"`
	Body                 string            `json:"body,omitempty" jsonschema:"description=Request body"`
	ExpectedStatus       int               `json:"expected_status,omitempty" jsonschema:"default=200,description=Expected HTTP status code"`
	ExpectedBodyContains string            `json:"expected_body_contains,omitempty" jsonschema:"description=String that should be present in response body"`
	BodyPreviewLength    int               `json:"body_preview_length,omitempty" jsonschema:"default=200,description=Number of characters to include from response body (0 = hash only, -1 = full body)"`
	FollowRedirects      bool              `json:"follow_redirects,omitempty" jsonschema:"default=true,description=Follow HTTP redirects"`
	TimeoutSeconds       int               `json:"timeout_seconds,omitempty" jsonschema:"default=30,minimum=1,maximum=300,description=Request timeout in seconds"`
}
