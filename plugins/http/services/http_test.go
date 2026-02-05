package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHTTPClient implements ports.HTTPClient for testing
type MockHTTPClient struct {
	client *http.Client
}

func (c *MockHTTPClient) Do(ctx context.Context, req ports.HTTPRequest) (*ports.HTTPResponse, error) {
	stdReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	for k, v := range req.Headers {
		stdReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(stdReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string][]string)
	for k, v := range resp.Header {
		headers[k] = v
	}

	return &ports.HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
		Proto:      resp.Proto,
	}, nil
}

func (c *MockHTTPClient) Get(ctx context.Context, url string) (*ports.HTTPResponse, error) {
	return c.Do(ctx, ports.HTTPRequest{Method: "GET", URL: url})
}

func (c *MockHTTPClient) Post(ctx context.Context, url string, contentType string, body []byte) (*ports.HTTPResponse, error) {
	return c.Do(ctx, ports.HTTPRequest{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		Body: body,
	})
}

func TestExamples(t *testing.T) {
	// Example tests require network access to external services
	// Run with: go test -tags=integration
	t.Skip("Example tests require network access - run with -tags=integration")
}

func TestHTTPService_Get_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	input := &RequestInput{
		URL:    server.URL,
		Method: "GET",
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.HTTPClient(mockClient))

	output, err := svc.RequestHandler(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, server.URL, output.URL)
	assert.Equal(t, "GET", output.Method)
	assert.Equal(t, 200, output.StatusCode)
	assert.Contains(t, output.Status, "200")
	assert.Equal(t, "hello world", output.Body)
	assert.Equal(t, 11, output.BodySize)
	assert.GreaterOrEqual(t, output.ResponseTimeMs, int64(0))
}

func TestHTTPService_Post_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	input := &RequestInput{
		URL:    server.URL,
		Method: "POST",
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.HTTPClient(mockClient))

	output, err := svc.RequestHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "POST", output.Method)
	assert.Equal(t, 200, output.StatusCode)
}

func TestHTTPService_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	input := &RequestInput{
		URL:    server.URL,
		Method: "GET",
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.HTTPClient(mockClient))

	// Handler returns success with 404 code - validation happens via expect expressions
	output, err := svc.RequestHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, 404, output.StatusCode)
	assert.Contains(t, output.Status, "404")
}

func TestHTTPService_DefaultMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	input := &RequestInput{
		URL: server.URL,
		// Method not specified - should default to GET
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.HTTPClient(mockClient))

	output, err := svc.RequestHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "GET", output.Method)
}
