package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/http/core"
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

func TestHTTPService_Get_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	cfg := &core.HTTPConfig{
		URL: server.URL,
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.GetHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, 200, result.Data["status_code"])
}

func TestHTTPService_Get_ExpectedStatus_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	cfg := &core.HTTPConfig{
		URL:            server.URL,
		ExpectedStatus: 201,
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.GetHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Equal(t, 200, result.Data["status_code"])
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
	cfg := &core.HTTPConfig{
		URL:    server.URL,
		Method: "POST",
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.PostHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, 200, result.Data["status_code"])
}

func TestHTTPService_CheckStatus_MethodOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	cfg := &core.HTTPConfig{
		URL:    server.URL,
		Method: "HEAD",
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.CheckStatusHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}

func TestHTTPService_CheckSSL_Success(t *testing.T) {
	// CheckSSL uses HEAD method internally
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	cfg := &core.HTTPConfig{
		URL: server.URL,
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.CheckSSLHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, "HEAD", result.Data["method"])
}

func TestHTTPService_Get_BodyContains_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "version": "1.0.0"}`))
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	cfg := &core.HTTPConfig{
		URL:                  server.URL,
		ExpectedBodyContains: "healthy",
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.GetHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}

func TestHTTPService_Get_BodyContains_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "degraded"}`))
	}))
	defer server.Close()

	svc := &HTTPService{}
	mockClient := &MockHTTPClient{client: server.Client()}
	cfg := &core.HTTPConfig{
		URL:                  server.URL,
		ExpectedBodyContains: "healthy",
	}

	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.GetHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Contains(t, result.Message, "missing expected content")
}
