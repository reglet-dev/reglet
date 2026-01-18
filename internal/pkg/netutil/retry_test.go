package netutil_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

// mockTransport is a test double for http.RoundTripper.
type mockTransport struct {
	responses []*http.Response
	errors    []error
	calls     int
}

func (m *mockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	idx := m.calls
	m.calls++

	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func Test_RetryTransport_SuccessFirstAttempt(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
		},
	}

	transport := &netutil.RetryTransport{
		Base:       mock,
		MaxRetries: 3,
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := transport.RoundTrip(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, mock.calls)
}

func Test_RetryTransport_Retries429(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}},
			{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}},
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
		},
	}

	var retryAttempts []int
	transport := &netutil.RetryTransport{
		Base:           mock,
		MaxRetries:     3,
		InitialBackoff: time.Millisecond, // Fast for tests
		OnRetry: func(attempt int, _ time.Duration, _ int) {
			retryAttempts = append(retryAttempts, attempt)
		},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := transport.RoundTrip(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, mock.calls)
	assert.Equal(t, []int{1, 2}, retryAttempts)
}

func Test_RetryTransport_Retries5xx(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
		{"504 Gateway Timeout", http.StatusGatewayTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTransport{
				responses: []*http.Response{
					{StatusCode: tt.statusCode, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}},
					{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
				},
			}

			transport := &netutil.RetryTransport{
				Base:           mock,
				MaxRetries:     3,
				InitialBackoff: time.Millisecond,
			}

			req, _ := http.NewRequest("GET", "http://example.com", nil)
			resp, err := transport.RoundTrip(req)

			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, 2, mock.calls)
		})
	}
}

func Test_RetryTransport_NoRetryOn4xx(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTransport{
				responses: []*http.Response{
					{StatusCode: tt.statusCode, Body: io.NopCloser(strings.NewReader(""))},
				},
			}

			transport := &netutil.RetryTransport{
				Base:           mock,
				MaxRetries:     3,
				InitialBackoff: time.Millisecond,
			}

			req, _ := http.NewRequest("GET", "http://example.com", nil)
			resp, err := transport.RoundTrip(req)

			require.NoError(t, err)
			assert.Equal(t, tt.statusCode, resp.StatusCode)
			assert.Equal(t, 1, mock.calls) // No retries
		})
	}
}

func Test_RetryTransport_RespectsRetryAfterHeader(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Retry-After": []string{"1"}}, // 1 second
			},
			{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))},
		},
	}

	var waitDuration time.Duration
	transport := &netutil.RetryTransport{
		Base:           mock,
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		OnRetry: func(_ int, d time.Duration, _ int) {
			waitDuration = d
		},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, _ = transport.RoundTrip(req)

	// Should use the Retry-After value (1 second)
	assert.Equal(t, time.Second, waitDuration)
}

func Test_IsRetryableStatus(t *testing.T) {
	assert.True(t, netutil.IsRetryableStatus(429))
	assert.True(t, netutil.IsRetryableStatus(502))
	assert.True(t, netutil.IsRetryableStatus(503))
	assert.True(t, netutil.IsRetryableStatus(504))
	assert.False(t, netutil.IsRetryableStatus(200))
	assert.False(t, netutil.IsRetryableStatus(400))
	assert.False(t, netutil.IsRetryableStatus(401))
	assert.False(t, netutil.IsRetryableStatus(404))
	assert.False(t, netutil.IsRetryableStatus(500))
}
