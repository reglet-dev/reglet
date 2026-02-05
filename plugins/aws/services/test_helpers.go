package services

import (
	"context"

	"github.com/reglet-dev/reglet-sdk/domain/ports"
)

// MockHTTPClient implements ports.HTTPClient for testing
type MockHTTPClient struct {
	Response *ports.HTTPResponse
	Err      error
}

func (m *MockHTTPClient) Do(ctx context.Context, req ports.HTTPRequest) (*ports.HTTPResponse, error) {
	return m.Response, m.Err
}

func (m *MockHTTPClient) Get(ctx context.Context, url string) (*ports.HTTPResponse, error) {
	return m.Response, m.Err
}

func (m *MockHTTPClient) Post(ctx context.Context, url string, contentType string, body []byte) (*ports.HTTPResponse, error) {
	return m.Response, m.Err
}
