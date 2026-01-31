package core

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	regletnet "github.com/reglet-dev/reglet-sdk/go/net"
)

// AWSClient handles AWS API requests.
type AWSClient struct {
	Creds      *AWSCredentials
	HTTPClient ports.HTTPClient
	Timeout    time.Duration
}

// NewAWSClient creates a new AWS client.
func NewAWSClient(creds *AWSCredentials, timeoutSeconds int) *AWSClient {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &AWSClient{
		Creds:   creds,
		Timeout: timeout,
	}
}

// GetHTTPClient returns the HTTP client (injectable for testing).
func (c *AWSClient) GetHTTPClient() ports.HTTPClient {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	// Use SDK's WASM HTTP transport
	return regletnet.NewTransport()
}

// Call makes a signed request to an AWS API.
func (c *AWSClient) Call(ctx context.Context, service, action string, params map[string]string) ([]byte, error) {
	// Build endpoint URL
	endpoint := c.getEndpoint(service)

	// Build query string
	queryParams := make(map[string]string)
	queryParams["Action"] = action
	queryParams["Version"] = c.getAPIVersion(service)
	for k, v := range params {
		queryParams[k] = v
	}

	// Build URL with query string
	urlStr := endpoint + "?" + buildQueryString(queryParams)

	// Create http.Request for signing
	// We use http.NewRequest because our signer works with it
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Sign the request
	if err := signRequest(req, c.Creds, service); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Convert to ports.HTTPRequest
	portReq := ports.HTTPRequest{
		Method:  req.Method,
		URL:     urlStr,
		Headers: make(map[string]string),
		Timeout: int(c.Timeout.Milliseconds()),
	}

	// Flatten headers: ports.HTTPRequest uses map[string]string (single value)
	for k, v := range req.Header {
		if len(v) > 0 {
			portReq.Headers[k] = v[0]
		}
	}

	// Execute request using SDK client
	resp, err := c.GetHTTPClient().Do(ctx, portReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for AWS errors
	if resp.StatusCode >= 400 {
		return nil, parseAWSError(resp.Body, resp.StatusCode)
	}

	return resp.Body, nil
}

// getEndpoint returns the AWS service endpoint URL.
func (c *AWSClient) getEndpoint(service string) string {
	switch service {
	case "iam":
		// IAM is a global service
		return "https://iam.amazonaws.com"
	case "s3":
		return fmt.Sprintf("https://s3.%s.amazonaws.com", c.Creds.Region)
	case "lambda":
		return fmt.Sprintf("https://lambda.%s.amazonaws.com", c.Creds.Region)
	default:
		// ec2, rds, vpc use standard regional endpoints
		return fmt.Sprintf("https://%s.%s.amazonaws.com", service, c.Creds.Region)
	}
}

// getAPIVersion returns the API version for a service.
func (c *AWSClient) getAPIVersion(service string) string {
	versions := map[string]string{
		"iam":    "2010-05-08",
		"ec2":    "2016-11-15",
		"s3":     "2006-03-01",
		"rds":    "2014-10-31",
		"lambda": "2015-03-31",
	}
	if v, ok := versions[service]; ok {
		return v
	}
	return "2016-11-15" // Default EC2 version
}

// buildQueryString creates a URL query string from parameters.
func buildQueryString(params map[string]string) string {
	var pairs []string
	for k, v := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, "&")
}

// AWSError represents an AWS API error response.
type AWSError struct {
	Code    string
	Message string
	Status  int
}

func (e *AWSError) Error() string {
	return fmt.Sprintf("AWS error %d: %s - %s", e.Status, e.Code, e.Message)
}

// parseAWSError parses an AWS error response.
func parseAWSError(body []byte, statusCode int) error {
	// Try XML format (most AWS APIs)
	var xmlErr struct {
		Error struct {
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		} `xml:"Error"`
	}
	if err := xml.Unmarshal(body, &xmlErr); err == nil && xmlErr.Error.Code != "" {
		return &AWSError{
			Code:    xmlErr.Error.Code,
			Message: xmlErr.Error.Message,
			Status:  statusCode,
		}
	}

	// Fallback
	return &AWSError{
		Code:    "UnknownError",
		Message: string(body),
		Status:  statusCode,
	}
}
