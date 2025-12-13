package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
)

func Test_StatusAggregator_AggregateControlStatus(t *testing.T) {
	tests := []struct {
		name     string
		statuses []values.Status
		expected values.Status
	}{
		{
			name:     "empty observations returns skipped",
			statuses: []values.Status{},
			expected: values.StatusSkipped,
		},
		{
			name: "all pass returns pass",
			statuses: []values.Status{
				values.StatusPass,
				values.StatusPass,
				values.StatusPass,
			},
			expected: values.StatusPass,
		},
		{
			name: "any failure returns fail (even with errors)",
			statuses: []values.Status{
				values.StatusPass,
				values.StatusFail,
				values.StatusError,
			},
			expected: values.StatusFail,
		},
		{
			name: "any error without failures returns error",
			statuses: []values.Status{
				values.StatusPass,
				values.StatusError,
				values.StatusPass,
			},
			expected: values.StatusError,
		},
		{
			name: "skipped observations don't affect pass",
			statuses: []values.Status{
				values.StatusPass,
				values.StatusSkipped,
				values.StatusPass,
			},
			expected: values.StatusPass,
		},
		{
			name: "all skipped returns skipped",
			statuses: []values.Status{
				values.StatusSkipped,
				values.StatusSkipped,
			},
			expected: values.StatusSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := NewStatusAggregator()
			result := aggregator.AggregateControlStatus(tt.statuses)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_StatusAggregator_DetermineObservationStatus(t *testing.T) {
	tests := []struct {
		name           string
		evidence       *execution.Evidence
		expects        []string
		expectedStatus values.Status
		expectedError  string
	}{
		{
			name: "no expects uses evidence status true",
			evidence: &execution.Evidence{
				Status: true,
				Data:   map[string]interface{}{},
			},
			expects:        []string{},
			expectedStatus: values.StatusPass,
		},
		{
			name: "no expects uses evidence status false",
			evidence: &execution.Evidence{
				Status: false,
				Data:   map[string]interface{}{},
			},
			expects:        []string{},
			expectedStatus: values.StatusFail,
		},
		{
			name: "simple expect passes",
			evidence: &execution.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 200,
				},
			},
			expects:        []string{"data.status_code == 200"},
			expectedStatus: values.StatusPass,
		},
		{
			name: "simple expect fails",
			evidence: &execution.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 500,
				},
			},
			expects:        []string{"data.status_code == 200"},
			expectedStatus: values.StatusFail,
			expectedError:  "expectation failed: data.status_code == 200",
		},
		{
			name: "multiple expects all pass",
			evidence: &execution.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 200,
					"connected":   true,
				},
			},
			expects:        []string{"data.status_code == 200", "data.connected == true"},
			expectedStatus: values.StatusPass,
		},
		{
			name: "any expect fails results in fail",
			evidence: &execution.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 200,
					"connected":   false,
				},
			},
			expects:        []string{"data.status_code == 200", "data.connected == true"},
			expectedStatus: values.StatusFail,
			expectedError:  "expectation failed: data.connected == true",
		},
		{
			name: "invalid expect expression returns error",
			evidence: &execution.Evidence{
				Status: true,
				Data:   map[string]interface{}{},
			},
			expects:        []string{"invalid syntax ==="},
			expectedStatus: values.StatusError,
		},
		{
			name: "evidence error skips expect evaluation",
			evidence: &execution.Evidence{
				Status: false,
				Error:  &execution.PluginError{Message: "connection failed"},
				Data:   map[string]interface{}{},
			},
			expects:        []string{"some_field == true"},
			expectedStatus: values.StatusError,
			expectedError:  "connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := NewStatusAggregator()
			status, errMsg := aggregator.DetermineObservationStatus(context.Background(), tt.evidence, tt.expects)

			assert.Equal(t, tt.expectedStatus, status)
			if tt.expectedError != "" {
				assert.Contains(t, errMsg, tt.expectedError)
			}
		})
	}
}
