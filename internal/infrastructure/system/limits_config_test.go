package system_test

import (
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/constants"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

func TestLimitsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *system.LimitsConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config is valid",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "empty config is valid",
			config:  &system.LimitsConfig{},
			wantErr: false,
		},
		{
			name: "all values within limits",
			config: &system.LimitsConfig{
				MaxEvidenceSize:           intPtr(1024 * 1024),
				MaxHTTPResponseSize:       intPtr(5 * 1024 * 1024),
				MaxCommandOutputSize:      intPtr(10 * 1024 * 1024),
				MaxSARIFArtifactSize:      intPtr(512 * 1024),
				MaxExpressionLength:       intPtr(500),
				MaxASTNodes:               intPtr(50),
				MaxHTTPRedirects:          intPtr(5),
				HTTPTimeout:               durationPtr(30 * time.Second),
				HTTPIdleTimeout:           durationPtr(90 * time.Second),
				MaxConcurrentControls:     intPtr(10),
				MaxConcurrentObservations: intPtr(5),
			},
			wantErr: false,
		},
		{
			name: "evidence size exceeds absolute maximum",
			config: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(constants.AbsoluteMaxEvidenceSize + 1),
			},
			wantErr: true,
			errMsg:  "max_evidence_size",
		},
		{
			name: "HTTP response size exceeds absolute maximum",
			config: &system.LimitsConfig{
				MaxHTTPResponseSize: intPtr(constants.AbsoluteMaxHTTPResponseSize + 1),
			},
			wantErr: true,
			errMsg:  "max_http_response_size",
		},
		{
			name: "expression length exceeds absolute maximum",
			config: &system.LimitsConfig{
				MaxExpressionLength: intPtr(constants.AbsoluteMaxExpressionLength + 1),
			},
			wantErr: true,
			errMsg:  "max_expression_length",
		},
		{
			name: "AST nodes exceeds absolute maximum",
			config: &system.LimitsConfig{
				MaxASTNodes: intPtr(constants.AbsoluteMaxASTNodes + 1),
			},
			wantErr: true,
			errMsg:  "max_ast_nodes",
		},
		{
			name: "HTTP timeout exceeds absolute maximum",
			config: &system.LimitsConfig{
				HTTPTimeout: durationPtr(constants.AbsoluteMaxHTTPTimeout + time.Second),
			},
			wantErr: true,
			errMsg:  "http_timeout",
		},
		{
			name: "concurrent controls exceeds absolute maximum",
			config: &system.LimitsConfig{
				MaxConcurrentControls: intPtr(constants.AbsoluteMaxConcurrentControls + 1),
			},
			wantErr: true,
			errMsg:  "max_concurrent_controls",
		},
		{
			name: "negative value rejected",
			config: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(-1),
			},
			wantErr: true,
			errMsg:  "must be non-negative",
		},
		{
			name: "negative duration rejected",
			config: &system.LimitsConfig{
				HTTPTimeout: durationPtr(-1 * time.Second),
			},
			wantErr: true,
			errMsg:  "must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestLimitsConfig_Merge(t *testing.T) {
	tests := []struct {
		name     string
		base     *system.LimitsConfig
		override *system.LimitsConfig
		want     *system.LimitsConfig
	}{
		{
			name:     "nil base, nil override",
			base:     nil,
			override: nil,
			want:     nil,
		},
		{
			name: "override single value",
			base: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(1024 * 1024),
				MaxASTNodes:     intPtr(50),
			},
			override: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(2 * 1024 * 1024), // Override
			},
			want: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(2 * 1024 * 1024), // From override
				MaxASTNodes:     intPtr(50),              // From base
			},
		},
		{
			name: "override all values",
			base: &system.LimitsConfig{
				MaxEvidenceSize:     intPtr(1024 * 1024),
				MaxHTTPResponseSize: intPtr(5 * 1024 * 1024),
			},
			override: &system.LimitsConfig{
				MaxEvidenceSize:     intPtr(2 * 1024 * 1024),
				MaxHTTPResponseSize: intPtr(10 * 1024 * 1024),
			},
			want: &system.LimitsConfig{
				MaxEvidenceSize:     intPtr(2 * 1024 * 1024),
				MaxHTTPResponseSize: intPtr(10 * 1024 * 1024),
			},
		},
		{
			name: "nil override returns base",
			base: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(1024 * 1024),
			},
			override: nil,
			want: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(1024 * 1024),
			},
		},
		{
			name: "nil base returns override",
			base: nil,
			override: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(2 * 1024 * 1024),
			},
			want: &system.LimitsConfig{
				MaxEvidenceSize: intPtr(2 * 1024 * 1024),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.Merge(tt.override)

			// Compare nil cases
			if (got == nil) != (tt.want == nil) {
				t.Errorf("Merge() = %v, want %v", got, tt.want)
				return
			}
			if got == nil {
				return
			}

			// Compare values
			if !compareLimitsConfig(got, tt.want) {
				t.Errorf("Merge() result doesn't match expected")
				t.Logf("got:  %+v", got)
				t.Logf("want: %+v", tt.want)
			}
		})
	}
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func compareLimitsConfig(a, b *system.LimitsConfig) bool {
	return compareIntPtr(a.MaxEvidenceSize, b.MaxEvidenceSize) &&
		compareIntPtr(a.MaxHTTPResponseSize, b.MaxHTTPResponseSize) &&
		compareIntPtr(a.MaxCommandOutputSize, b.MaxCommandOutputSize) &&
		compareIntPtr(a.MaxSARIFArtifactSize, b.MaxSARIFArtifactSize) &&
		compareIntPtr(a.MaxExpressionLength, b.MaxExpressionLength) &&
		compareIntPtr(a.MaxASTNodes, b.MaxASTNodes) &&
		compareIntPtr(a.MaxHTTPRedirects, b.MaxHTTPRedirects) &&
		compareDurationPtr(a.HTTPTimeout, b.HTTPTimeout) &&
		compareDurationPtr(a.HTTPIdleTimeout, b.HTTPIdleTimeout) &&
		compareIntPtr(a.MaxConcurrentControls, b.MaxConcurrentControls) &&
		compareIntPtr(a.MaxConcurrentObservations, b.MaxConcurrentObservations)
}

func compareIntPtr(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func compareDurationPtr(a, b *time.Duration) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
