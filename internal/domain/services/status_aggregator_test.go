package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/engine"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
)

func Test_StatusAggregator_AggregateControlStatus(t *testing.T) {
	tests := []struct {
		name         string
		observations []engine.ObservationResult
		expected     engine.Status
	}{
		{
			name:         "empty observations returns skipped",
			observations: []engine.ObservationResult{},
			expected:     engine.StatusSkipped,
		},
		{
			name: "all pass returns pass",
			observations: []engine.ObservationResult{
				{Status: engine.StatusPass},
				{Status: engine.StatusPass},
				{Status: engine.StatusPass},
			},
			expected: engine.StatusPass,
		},
		{
			name: "any failure returns fail (even with errors)",
			observations: []engine.ObservationResult{
				{Status: engine.StatusPass},
				{Status: engine.StatusFail},
				{Status: engine.StatusError},
			},
			expected: engine.StatusFail,
		},
		{
			name: "any error without failures returns error",
			observations: []engine.ObservationResult{
				{Status: engine.StatusPass},
				{Status: engine.StatusError},
				{Status: engine.StatusPass},
			},
			expected: engine.StatusError,
		},
		{
			name: "skipped observations don't affect pass",
			observations: []engine.ObservationResult{
				{Status: engine.StatusPass},
				{Status: engine.StatusSkipped},
				{Status: engine.StatusPass},
			},
			expected: engine.StatusPass,
		},
		{
			name: "all skipped returns skipped",
			observations: []engine.ObservationResult{
				{Status: engine.StatusSkipped},
				{Status: engine.StatusSkipped},
			},
			expected: engine.StatusSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := NewStatusAggregator()
			result := aggregator.AggregateControlStatus(tt.observations)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_StatusAggregator_DetermineObservationStatus(t *testing.T) {
	tests := []struct {
		name           string
		evidence       *wasm.Evidence
		expects        []string
		expectedStatus engine.Status
		expectedError  string
	}{
		{
			name: "no expects uses evidence status true",
			evidence: &wasm.Evidence{
				Status: true,
				Data:   map[string]interface{}{},
			},
			expects:        []string{},
			expectedStatus: engine.StatusPass,
		},
		{
			name: "no expects uses evidence status false",
			evidence: &wasm.Evidence{
				Status: false,
				Data:   map[string]interface{}{},
			},
			expects:        []string{},
			expectedStatus: engine.StatusFail,
		},
		{
			name: "simple expect passes",
			evidence: &wasm.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 200,
				},
			},
			expects:        []string{"status_code == 200"},
			expectedStatus: engine.StatusPass,
		},
		{
			name: "simple expect fails",
			evidence: &wasm.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 500,
				},
			},
			expects:        []string{"status_code == 200"},
			expectedStatus: engine.StatusFail,
			expectedError:  "expectation failed: status_code == 200",
		},
		{
			name: "multiple expects all pass",
			evidence: &wasm.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 200,
					"connected":   true,
				},
			},
			expects:        []string{"status_code == 200", "connected == true"},
			expectedStatus: engine.StatusPass,
		},
		{
			name: "any expect fails results in fail",
			evidence: &wasm.Evidence{
				Status: true,
				Data: map[string]interface{}{
					"status_code": 200,
					"connected":   false,
				},
			},
			expects:        []string{"status_code == 200", "connected == true"},
			expectedStatus: engine.StatusFail,
			expectedError:  "expectation failed: connected == true",
		},
		{
			name: "invalid expect expression returns error",
			evidence: &wasm.Evidence{
				Status: true,
				Data:   map[string]interface{}{},
			},
			expects:        []string{"invalid syntax ==="},
			expectedStatus: engine.StatusError,
		},
		{
			name: "evidence error skips expect evaluation",
			evidence: &wasm.Evidence{
				Status: false,
				Error:  &wasm.PluginError{Message: "connection failed"},
				Data:   map[string]interface{}{},
			},
			expects:        []string{"some_field == true"},
			expectedStatus: engine.StatusError,
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
