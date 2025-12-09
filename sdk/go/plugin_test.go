//go:build !wasip1

package sdk

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata_Capabilities(t *testing.T) {
	tests := []struct {
		name         string
		metadata     Metadata
		wantJSON     string
		capabilities []Capability
	}{
		{
			name: "single capability",
			metadata: Metadata{
				Name:    "test",
				Version: "1.0.0",
				Capabilities: []Capability{
					{Kind: "fs", Pattern: "read:/etc/**"},
				},
			},
			capabilities: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
		},
		{
			name: "multiple capabilities",
			metadata: Metadata{
				Name:    "test",
				Version: "1.0.0",
				Capabilities: []Capability{
					{Kind: "network", Pattern: "outbound:80,443"},
					{Kind: "exec", Pattern: "systemctl"},
				},
			},
			capabilities: []Capability{
				{Kind: "network", Pattern: "outbound:80,443"},
				{Kind: "exec", Pattern: "systemctl"},
			},
		},
		{
			name: "no capabilities",
			metadata: Metadata{
				Name:         "test",
				Version:      "1.0.0",
				Capabilities: []Capability{},
			},
			capabilities: []Capability{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.capabilities, tt.metadata.Capabilities)

			// Test JSON serialization
			data, err := json.Marshal(tt.metadata)
			require.NoError(t, err)

			var decoded Metadata
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.metadata.Name, decoded.Name)
			assert.Equal(t, tt.metadata.Version, decoded.Version)
			assert.Equal(t, len(tt.metadata.Capabilities), len(decoded.Capabilities))
		})
	}
}

func TestEvidence_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		evidence Evidence
	}{
		{
			name: "success evidence",
			evidence: Evidence{
				Status: true,
				Data: map[string]interface{}{
					"stdout": "hello world",
					"exit_code": 0,
				},
			},
		},
		{
			name: "failure evidence with error",
			evidence: Evidence{
				Status: false,
				Data:   map[string]interface{}{"attempted": true},
				Error: &ErrorDetail{
					Message: "connection refused",
					Type:    "network",
					Code:    "ECONNREFUSED",
				},
			},
		},
		{
			name: "evidence with nil error",
			evidence: Evidence{
				Status: true,
				Data:   map[string]interface{}{"result": "ok"},
				Error:  nil,
			},
		},
		{
			name: "evidence with stack trace",
			evidence: Evidence{
				Status: false,
				Error: &ErrorDetail{
					Message: "panic occurred",
					Type:    "panic",
					Stack:   []byte("goroutine 1 [running]:\nmain.go:10"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshal/unmarshal round-trip
			data, err := json.Marshal(tt.evidence)
			require.NoError(t, err)

			var decoded Evidence
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.evidence.Status, decoded.Status)

			if tt.evidence.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.evidence.Error.Message, decoded.Error.Message)
				assert.Equal(t, tt.evidence.Error.Type, decoded.Error.Type)
				assert.Equal(t, tt.evidence.Error.Code, decoded.Error.Code)
			} else {
				assert.Nil(t, decoded.Error)
			}
		})
	}
}

func TestConfig_Handling(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantJSON string
	}{
		{
			name: "simple config",
			config: Config{
				"host": "example.com",
				"port": 443,
			},
			wantJSON: `{"host":"example.com","port":443}`,
		},
		{
			name: "nested config",
			config: Config{
				"server": map[string]interface{}{
					"host": "example.com",
					"port": 443,
				},
				"timeout": 30,
			},
		},
		{
			name:     "empty config",
			config:   Config{},
			wantJSON: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON serialization
			data, err := json.Marshal(tt.config)
			require.NoError(t, err)

			// Test JSON deserialization
			var decoded Config
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// For simple configs, check exact JSON match
			if tt.wantJSON != "" {
				assert.JSONEq(t, tt.wantJSON, string(data))
			}
		})
	}
}

func TestToErrorDetail(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantMessage string
		wantType    string
	}{
		{
			name:        "simple error",
			err:         fmt.Errorf("connection failed"),
			wantMessage: "connection failed",
			wantType:    "internal",
		},
		{
			name:        "wrapped error",
			err:         fmt.Errorf("failed to connect: %w", fmt.Errorf("timeout")),
			wantMessage: "failed to connect: timeout",
			wantType:    "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := ToErrorDetail(tt.err)
			require.NotNil(t, detail)
			assert.Equal(t, tt.wantMessage, detail.Message)
			assert.Equal(t, tt.wantType, detail.Type)
		})
	}
}

func TestSuccessHelper(t *testing.T) {
	data := map[string]interface{}{
		"result": "ok",
		"count":  42,
	}

	evidence := Success(data)
	assert.True(t, evidence.Status)
	assert.Equal(t, data, evidence.Data)
	assert.Nil(t, evidence.Error)
}

func TestFailureHelper(t *testing.T) {
	tests := []struct {
		name        string
		errType     string
		message     string
		wantStatus  bool
		wantMessage string
	}{
		{
			name:        "network failure",
			errType:     "network",
			message:     "connection refused",
			wantStatus:  false,
			wantMessage: "connection refused",
		},
		{
			name:        "validation failure",
			errType:     "validation",
			message:     "invalid input",
			wantStatus:  false,
			wantMessage: "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := Failure(tt.errType, tt.message)
			assert.False(t, evidence.Status)
			require.NotNil(t, evidence.Error)
			assert.Equal(t, tt.wantMessage, evidence.Error.Message)
			assert.Equal(t, tt.errType, evidence.Error.Type)
		})
	}
}

func TestConfigErrorHelper(t *testing.T) {
	err := fmt.Errorf("missing required field 'host'")
	evidence := ConfigError(err)

	assert.False(t, evidence.Status)
	require.NotNil(t, evidence.Error)
	assert.Contains(t, evidence.Error.Message, "missing required field")
	// Note: ConfigError currently uses ToErrorDetail which returns "internal" type
	// This will be improved in Phase 4 when we add custom error types
	assert.Equal(t, "internal", evidence.Error.Type)
}

func TestNetworkErrorHelper(t *testing.T) {
	err := fmt.Errorf("connection timeout")
	evidence := NetworkError("failed to connect to api.example.com:443", err)

	assert.False(t, evidence.Status)
	require.NotNil(t, evidence.Error)
	assert.Contains(t, evidence.Error.Message, "failed to connect")
	assert.Equal(t, "network", evidence.Error.Type)

	// Test that wrapped error is populated
	assert.NotNil(t, evidence.Error.Wrapped)
	assert.Contains(t, evidence.Error.Wrapped.Message, "connection timeout")
}
