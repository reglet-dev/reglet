package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/engine"
)

func TestValidateFilterConfig(t *testing.T) {
	// Save and restore global filterExpr
	originalFilterExpr := filterExpr
	defer func() { filterExpr = originalFilterExpr }()

	// Create a dummy profile
	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "c1"},
				{ID: "c2"},
			},
		},
	}

	tests := []struct {
		name          string
		includeIDs    []string
		excludeIDs    []string
		filterExprVal string
		wantErr       bool
		errMsg        string
	}{
		{
			name:       "valid-control-ids",
			includeIDs: []string{"c1", "c2"},
			wantErr:    false,
		},
		{
			name:       "invalid-control-id-include",
			includeIDs: []string{"c1", "non-existent"},
			wantErr:    true,
			errMsg:     "--control references non-existent control: non-existent",
		},
		{
			name:       "valid-exclude-ids",
			excludeIDs: []string{"c1"},
			wantErr:    false,
		},
		{
			name:       "invalid-control-id-exclude",
			excludeIDs: []string{"c1", "non-existent"},
			wantErr:    true,
			errMsg:     "--exclude-control references non-existent control: non-existent",
		},
		{
			name:          "valid-filter-expr",
			filterExprVal: "severity == 'high'",
			wantErr:       false,
		},
		{
			name:          "invalid-filter-expr",
			filterExprVal: "invalid syntax ((",
			wantErr:       true,
			errMsg:        "invalid --filter expression",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset globals
			filterExpr = tt.filterExprVal

			cfg := engine.DefaultExecutionConfig()
			cfg.IncludeControlIDs = tt.includeIDs
			cfg.ExcludeControlIDs = tt.excludeIDs

			err := validateFilterConfig(profile, &cfg)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				if tt.filterExprVal != "" {
					assert.NotNil(t, cfg.FilterProgram, "FilterProgram should be compiled")
				}
			}
		})
	}
}
