package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/aws/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestHandleGetAccountSummary_MFAEnabled(t *testing.T) {
	// Mock AWS response
	mockResponseXML := `<?xml version="1.0" encoding="UTF-8"?>
    <GetAccountSummaryResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
        <GetAccountSummaryResult>
            <SummaryMap>
                <entry><key>AccountMFAEnabled</key><value>1</value></entry>
                <entry><key>Users</key><value>10</value></entry>
            </SummaryMap>
        </GetAccountSummaryResult>
    </GetAccountSummaryResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockResponseXML),
		},
	}

	// Create client with mock
	creds := &core.AWSCredentials{
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		Region:          "us-east-1",
	}
	client := core.NewAWSClient(creds, 30)
	client.HTTPClient = mockClient

	cfg := &core.AWSConfig{Service: "iam", Operation: "get_account_summary"}

	result, err := handleGetAccountSummary(context.Background(), client, cfg)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.True(t, result.Data["root_mfa_enabled"].(bool))
}

func TestHandleGetAccountSummary_MFADisabled(t *testing.T) {
	mockResponseXML := `<?xml version="1.0" encoding="UTF-8"?>
    <GetAccountSummaryResponse>
        <GetAccountSummaryResult>
            <SummaryMap>
                <entry><key>AccountMFAEnabled</key><value>0</value></entry>
            </SummaryMap>
        </GetAccountSummaryResult>
    </GetAccountSummaryResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockResponseXML),
		},
	}

	creds := &core.AWSCredentials{AccessKeyID: "test", SecretAccessKey: "test", Region: "us-east-1"}
	client := core.NewAWSClient(creds, 30)
	client.HTTPClient = mockClient

	result, err := handleGetAccountSummary(context.Background(), client, &core.AWSConfig{})
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.False(t, result.Data["root_mfa_enabled"].(bool))
}
