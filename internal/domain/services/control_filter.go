package services

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// ControlEnv defines the variables available during filter expression evaluation.
type ControlEnv struct {
	ID       string   `expr:"id"`
	Name     string   `expr:"name"`
	Severity string   `expr:"severity"`
	Owner    string   `expr:"owner"`
	Tags     []string `expr:"tags"`
}

// ControlFilter implements policy selection logic based on tags, severity, and IDs.
type ControlFilter struct {
	// Exclusive mode: only include specified controls
	exclusiveControlIDs map[string]bool

	// Exclusion filters
	excludeControlIDs map[string]bool
	excludeTags       map[string]bool

	// Inclusion filters
	includeTags       map[string]bool
	includeSeverities map[string]bool

	// Advanced filtering
	filterProgram *vm.Program
}

// NewControlFilter initializes a new empty filter.
func NewControlFilter() *ControlFilter {
	return &ControlFilter{
		exclusiveControlIDs: make(map[string]bool),
		excludeControlIDs:   make(map[string]bool),
		excludeTags:         make(map[string]bool),
		includeTags:         make(map[string]bool),
		includeSeverities:   make(map[string]bool),
	}
}

// WithExclusiveControls restricts execution to ONLY the specified control IDs.
// If set, all other filters are ignored.
func (f *ControlFilter) WithExclusiveControls(controlIDs []string) *ControlFilter {
	f.exclusiveControlIDs = toSet(controlIDs)
	return f
}

// WithExcludedControls excludes specific control IDs.
func (f *ControlFilter) WithExcludedControls(controlIDs []string) *ControlFilter {
	f.excludeControlIDs = toSet(controlIDs)
	return f
}

// WithExcludedTags excludes controls with any of these tags.
func (f *ControlFilter) WithExcludedTags(tags []string) *ControlFilter {
	f.excludeTags = toSet(tags)
	return f
}

// WithIncludedTags includes only controls with any of these tags.
func (f *ControlFilter) WithIncludedTags(tags []string) *ControlFilter {
	f.includeTags = toSet(tags)
	return f
}

// WithIncludedSeverities includes only controls with these severities.
func (f *ControlFilter) WithIncludedSeverities(severities []string) *ControlFilter {
	f.includeSeverities = toSet(severities)
	return f
}

// WithFilterExpression applies a compiled Expr program for advanced filtering.
func (f *ControlFilter) WithFilterExpression(program *vm.Program) *ControlFilter {
	f.filterProgram = program
	return f
}

// ShouldRun evaluates whether a control matches the filter criteria.
// It returns true if the control should execute, along with a reason if skipped.
func (f *ControlFilter) ShouldRun(ctrl entities.Control) (bool, string) {
	// 0. Exclusive mode: ONLY specified controls run
	if len(f.exclusiveControlIDs) > 0 {
		if f.exclusiveControlIDs[ctrl.ID] {
			return true, ""
		}
		return false, "excluded by --control filter"
	}

	// 1. Exclude by control ID
	if f.excludeControlIDs[ctrl.ID] {
		return false, "excluded by --exclude-control"
	}

	// 2. Exclude by tags
	for _, tag := range ctrl.Tags {
		if f.excludeTags[tag] {
			return false, fmt.Sprintf("excluded by --exclude-tags %s", tag)
		}
	}

	// 3. Include by severity (if filter specified)
	if len(f.includeSeverities) > 0 {
		if !f.includeSeverities[ctrl.Severity] {
			return false, "excluded by --severity filter"
		}
	}

	// 4. Include by tags (if filter specified)
	if len(f.includeTags) > 0 {
		hasTag := false
		for _, tag := range ctrl.Tags {
			if f.includeTags[tag] {
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

// toSet converts a slice to a map (set)
func toSet(slice []string) map[string]bool {
	s := make(map[string]bool, len(slice))
	for _, item := range slice {
		s[item] = true
	}
	return s
}
