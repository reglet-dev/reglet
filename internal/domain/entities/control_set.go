package entities

import "fmt"

// ControlSet represents a collection of controls with domain logic for
// filtering, validation, and graph analysis.
type ControlSet []Control

// Validate checks the integrity of all controls in the set and ensures unique IDs.
func (cs ControlSet) Validate() error {
	controlIDs := make(map[string]bool)
	for i, ctrl := range cs {
		if err := ctrl.Validate(); err != nil {
			return fmt.Errorf("control %d (%s): %w", i, ctrl.ID, err)
		}

		if controlIDs[ctrl.ID] {
			return fmt.Errorf("duplicate control ID: %s", ctrl.ID)
		}
		controlIDs[ctrl.ID] = true
	}

	for _, ctrl := range cs {
		for _, dep := range ctrl.DependsOn {
			if !controlIDs[dep] {
				return fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, dep)
			}
		}
	}

	return cs.CheckForControlDependencyCycles()
}

// Add adds a control to the set. It ensures uniqueness and no cycles.
// It returns a new ControlSet with the added control.
// Note: This returns a NEW slice, it does not mutate the receiver if it was passed by value,
// but since it's a slice type, we should be careful.
// To match the existing immutable-ish pattern or safe add pattern, we will append and return.
func (cs ControlSet) Add(ctrl Control) (ControlSet, error) {
	if err := ctrl.Validate(); err != nil {
		return cs, fmt.Errorf("invalid control: %w", err)
	}

	for _, existing := range cs {
		if existing.ID == ctrl.ID {
			return cs, fmt.Errorf("control with ID %s already exists", ctrl.ID)
		}
	}

	// Verify dependencies exist
	// Note: We only check against EXISTING controls in the set.
	// If the control depends on itself, it will be caught here if we check ID==dep,
	// or in cycle check.
	// We need to check if dependencies specified in `ctrl` are already in `cs`.
	for _, dep := range ctrl.DependsOn {
		found := false
		for _, existing := range cs {
			if existing.ID == dep {
				found = true
				break
			}
		}
		if !found {
			return cs, fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, dep)
		}
	}

	// Tentatively add
	newSet := append(cs, ctrl) //nolint:gocritic // Intentionally creating new slice to avoid mutation effects until valid

	// Check for cycles
	if err := newSet.CheckForControlDependencyCycles(); err != nil {
		return cs, fmt.Errorf("adding control creates circular dependency: %w", err)
	}

	return newSet, nil
}

// Get returns a pointer to the control with the given ID, or nil if not found.
func (cs ControlSet) Get(id string) *Control {
	for i := range cs {
		if cs[i].ID == id {
			return &cs[i]
		}
	}
	return nil
}

// Has returns true if the control exists in the set.
func (cs ControlSet) Has(id string) bool {
	return cs.Get(id) != nil
}

// Count returns the number of controls.
func (cs ControlSet) Count() int {
	return len(cs)
}

// CheckForControlDependencyCycles checks if the control dependency graph contains any cycles.
func (cs ControlSet) CheckForControlDependencyCycles() error {
	// Build a map for O(1) lookups of dependencies
	controlDeps := make(map[string][]string)
	for _, ctrl := range cs {
		controlDeps[ctrl.ID] = ctrl.DependsOn
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	type stackFrame struct {
		id       string
		depIndex int
	}

	for _, ctrl := range cs {
		if visited[ctrl.ID] {
			continue
		}

		stack := []stackFrame{{id: ctrl.ID, depIndex: 0}}
		visited[ctrl.ID] = true
		recStack[ctrl.ID] = true

		for len(stack) > 0 {
			frame := &stack[len(stack)-1]
			deps := controlDeps[frame.id]

			if frame.depIndex < len(deps) {
				dep := deps[frame.depIndex]
				frame.depIndex++

				if recStack[dep] {
					return fmt.Errorf("circular dependency detected: %s -> %s", frame.id, dep)
				}

				if !visited[dep] {
					visited[dep] = true
					recStack[dep] = true
					stack = append(stack, stackFrame{id: dep, depIndex: 0})
				}
			} else {
				recStack[frame.id] = false
				stack = stack[:len(stack)-1]
			}
		}
	}

	return nil
}

// SelectByTags returns a subset of controls matching any of the specified tags.
func (cs ControlSet) SelectByTags(tags []string) ControlSet {
	if len(tags) == 0 {
		return cs
	}

	var selected ControlSet
	for _, ctrl := range cs {
		if ctrl.HasAnyTag(tags) {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// SelectBySeverity returns a subset of controls matching any of the specified severities.
func (cs ControlSet) SelectBySeverity(severities []string) ControlSet {
	if len(severities) == 0 {
		return cs
	}

	var selected ControlSet
	for _, ctrl := range cs {
		if ctrl.MatchesAnySeverity(severities) {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// ExcludeByID returns a subset of controls excluding the specified IDs.
func (cs ControlSet) ExcludeByID(excludeIDs []string) ControlSet {
	if len(excludeIDs) == 0 {
		return cs
	}

	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	var selected ControlSet
	for _, ctrl := range cs {
		if !excludeMap[ctrl.ID] {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// ApplyDefaults yields a new ControlSet with defaults applied to all controls.
// Note: Since ControlSet is a slice of values, modifying elements in place works if we
// are iterating by index, but it mutates the underlying array.
// To be safe and clean, we often mutate in place for "Apply" methods on the object itself.
func (cs ControlSet) ApplyDefaults(defaults *ControlDefaults) {
	for i := range cs {
		cs[i].ApplyDefaults(defaults)
	}
}
