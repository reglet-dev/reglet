package services

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// ControlSpecification defines a condition that a control must meet.
type ControlSpecification interface {
	// IsSatisfiedBy checks if the control meets the specification.
	// Returns true if satisfied, along with a reason if not (or empty if satisfied).
	IsSatisfiedBy(ctrl entities.Control) (bool, string)
}

// AndSpecification combines multiple specifications with logical AND.
type AndSpecification struct {
	specs []ControlSpecification
}

// NewAndSpecification creates a new AndSpecification.
func NewAndSpecification(specs ...ControlSpecification) *AndSpecification {
	return &AndSpecification{specs: specs}
}

// IsSatisfiedBy checks if all specifications are satisfied.
func (s *AndSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	for _, spec := range s.specs {
		if satisfied, reason := spec.IsSatisfiedBy(ctrl); !satisfied {
			return false, reason
		}
	}
	return true, ""
}

// ExclusiveControlsSpecification includes only specified control IDs.
type ExclusiveControlsSpecification struct {
	controlIDs map[string]bool
}

// NewExclusiveControlsSpecification creates a new ExclusiveControlsSpecification.
func NewExclusiveControlsSpecification(ids map[string]bool) *ExclusiveControlsSpecification {
	return &ExclusiveControlsSpecification{controlIDs: ids}
}

// IsSatisfiedBy checks if the control ID is in the exclusive list.
func (s *ExclusiveControlsSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	if len(s.controlIDs) == 0 {
		return true, "" // Not active
	}
	if s.controlIDs[ctrl.ID] {
		return true, ""
	}
	return false, "excluded by --control filter"
}

// ExcludedControlsSpecification excludes specified control IDs.
type ExcludedControlsSpecification struct {
	controlIDs map[string]bool
}

// NewExcludedControlsSpecification creates a new ExcludedControlsSpecification.
func NewExcludedControlsSpecification(ids map[string]bool) *ExcludedControlsSpecification {
	return &ExcludedControlsSpecification{controlIDs: ids}
}

// IsSatisfiedBy checks if the control ID is NOT in the excluded list.
func (s *ExcludedControlsSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	if s.controlIDs[ctrl.ID] {
		return false, "excluded by --exclude-control"
	}
	return true, ""
}

// ExcludedTagsSpecification excludes controls with any of the specified tags.
type ExcludedTagsSpecification struct {
	tags map[string]bool
}

// NewExcludedTagsSpecification creates a new ExcludedTagsSpecification.
func NewExcludedTagsSpecification(tags map[string]bool) *ExcludedTagsSpecification {
	return &ExcludedTagsSpecification{tags: tags}
}

// IsSatisfiedBy checks if the control has NONE of the excluded tags.
func (s *ExcludedTagsSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	for _, tag := range ctrl.Tags {
		if s.tags[tag] {
			return false, fmt.Sprintf("excluded by --exclude-tags %s", tag)
		}
	}
	return true, ""
}

// IncludedSeveritiesSpecification includes only controls with specified severities.
type IncludedSeveritiesSpecification struct {
	severities map[string]bool
}

// NewIncludedSeveritiesSpecification creates a new IncludedSeveritiesSpecification.
func NewIncludedSeveritiesSpecification(severities map[string]bool) *IncludedSeveritiesSpecification {
	return &IncludedSeveritiesSpecification{severities: severities}
}

// IsSatisfiedBy checks if the control severity is in the included list.
func (s *IncludedSeveritiesSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	if len(s.severities) == 0 {
		return true, ""
	}
	if !s.severities[ctrl.Severity] {
		return false, "excluded by --severity filter"
	}
	return true, ""
}

// IncludedTagsSpecification includes only controls with any of the specified tags.
type IncludedTagsSpecification struct {
	tags map[string]bool
}

// NewIncludedTagsSpecification creates a new IncludedTagsSpecification.
func NewIncludedTagsSpecification(tags map[string]bool) *IncludedTagsSpecification {
	return &IncludedTagsSpecification{tags: tags}
}

// IsSatisfiedBy checks if the control has ANY of the included tags.
func (s *IncludedTagsSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	if len(s.tags) == 0 {
		return true, ""
	}
	for _, tag := range ctrl.Tags {
		if s.tags[tag] {
			return true, ""
		}
	}
	return false, "excluded by --tags filter"
}

// ExpressionSpecification filters controls using an expr program.
type ExpressionSpecification struct {
	program *vm.Program
}

// NewExpressionSpecification creates a new ExpressionSpecification.
func NewExpressionSpecification(program *vm.Program) *ExpressionSpecification {
	return &ExpressionSpecification{program: program}
}

// IsSatisfiedBy evaluates the expr program against the control.
func (s *ExpressionSpecification) IsSatisfiedBy(ctrl entities.Control) (bool, string) {
	if s.program == nil {
		return true, ""
	}

	// Create evaluation environment
	// Note: ControlEnv is defined in control_filter.go (same package)
	env := ControlEnv{
		ID:       ctrl.ID,
		Name:     ctrl.Name,
		Severity: ctrl.Severity,
		Owner:    ctrl.Owner,
		Tags:     ctrl.Tags,
	}

	output, err := expr.Run(s.program, env)
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

	return true, ""
}
