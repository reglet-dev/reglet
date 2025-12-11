package services

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/whiskeyjimbo/reglet/internal/config"
)

// ControlEnv exposes control metadata for expression evaluation.
type ControlEnv struct {
	ID       string   `expr:"id"`
	Name     string   `expr:"name"`
	Severity string   `expr:"severity"`
	Owner    string   `expr:"owner"`
	Tags     []string `expr:"tags"`
}

// ControlFilter encapsulates all control filtering logic
type ControlFilter struct {
	// Exclusive mode: only include specified controls
	exclusiveControlIDs []string

	// Exclusion filters
	excludeControlIDs []string
	excludeTags       []string

	// Inclusion filters
	includeTags       []string
	includeSeverities []string

	// Advanced filtering via expression language
	filterProgram *vm.Program
}

// NewControlFilter creates a new control filter
func NewControlFilter() *ControlFilter {
	return &ControlFilter{}
}

// WithExclusiveControls sets exclusive mode (only these controls run)
func (f *ControlFilter) WithExclusiveControls(controlIDs []string) *ControlFilter {
	f.exclusiveControlIDs = controlIDs
	return f
}

// WithExcludedControls excludes specific control IDs
func (f *ControlFilter) WithExcludedControls(controlIDs []string) *ControlFilter {
	f.excludeControlIDs = controlIDs
	return f
}

// WithExcludedTags excludes controls with any of these tags
func (f *ControlFilter) WithExcludedTags(tags []string) *ControlFilter {
	f.excludeTags = tags
	return f
}

// WithIncludedTags includes only controls with any of these tags
func (f *ControlFilter) WithIncludedTags(tags []string) *ControlFilter {
	f.includeTags = tags
	return f
}

// WithIncludedSeverities includes only controls with these severities
func (f *ControlFilter) WithIncludedSeverities(severities []string) *ControlFilter {
	f.includeSeverities = severities
	return f
}

// WithFilterExpression sets an advanced filter expression
func (f *ControlFilter) WithFilterExpression(program *vm.Program) *ControlFilter {
	f.filterProgram = program
	return f
}

// ShouldRun determines if a control should execute based on filter criteria.
// Returns bool (should run) and string (skip reason if false).
//
// Filter Precedence (0-5):
// 0. EXCLUSIVE MODE: --control overrides all other filters
// 1. EXCLUSIONS: ExcludeControlIDs
// 2. EXCLUSIONS: ExcludeTags
// 3. INCLUDES: IncludeSeverities
// 4. INCLUDES: IncludeTags
// 5. ADVANCED: FilterProgram (expr language)
func (f *ControlFilter) ShouldRun(ctrl config.Control) (bool, string) {
	// 0. Exclusive mode: ONLY specified controls run
	if len(f.exclusiveControlIDs) > 0 {
		if contains(f.exclusiveControlIDs, ctrl.ID) {
			return true, ""
		}
		return false, "excluded by --control filter"
	}

	// 1. Exclude by control ID
	if contains(f.excludeControlIDs, ctrl.ID) {
		return false, "excluded by --exclude-control"
	}

	// 2. Exclude by tags
	for _, tag := range ctrl.Tags {
		if contains(f.excludeTags, tag) {
			return false, fmt.Sprintf("excluded by --exclude-tags %s", tag)
		}
	}

	// 3. Include by severity (if filter specified)
	if len(f.includeSeverities) > 0 {
		if !contains(f.includeSeverities, ctrl.Severity) {
			return false, "excluded by --severity filter"
		}
	}

	// 4. Include by tags (if filter specified)
	if len(f.includeTags) > 0 {
		hasTag := false
		for _, tag := range ctrl.Tags {
			if contains(f.includeTags, tag) {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false, "excluded by --tags filter"
		}
	}

	// 5. Advanced filter expression
	if f.filterProgram != nil {
		// Create evaluation environment
		env := ControlEnv{
			ID:       ctrl.ID,
			Name:     ctrl.Name,
			Severity: ctrl.Severity,
			Owner:    ctrl.Owner,
			Tags:     ctrl.Tags,
		}

		output, err := expr.Run(f.filterProgram, env)
		if err != nil {
			return false, fmt.Sprintf("filter expression error: %v", err)
		}

		result, ok := output.(bool)
		if !ok {
			return false, fmt.Sprintf("filter expression did not return boolean: %v", output)
		}

		if !result {
			return false, "excluded by --filter expression"
		}
	}

	// No filters matched - include by default
	return true, ""
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}