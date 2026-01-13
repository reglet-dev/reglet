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

// FilterConfig holds the configuration for filtering controls.
type FilterConfig struct {
	IncludeTags       []string
	IncludeSeverities []string
	IncludeIDs        []string
	ExcludeTags       []string
	ExcludeIDs        []string
}

// FilterOption is a functional option for configuring the filter.
type FilterOption func(*FilterConfig)

// WithTags filters controls to include only those with any of the specified tags.
func WithTags(tags ...string) FilterOption {
	return func(c *FilterConfig) {
		c.IncludeTags = append(c.IncludeTags, tags...)
	}
}

// WithSeverities filters controls to include only those with any of the specified severities.
func WithSeverities(severities ...string) FilterOption {
	return func(c *FilterConfig) {
		c.IncludeSeverities = append(c.IncludeSeverities, severities...)
	}
}

// WithIDs filters controls to include only those with the specified IDs.
func WithIDs(ids ...string) FilterOption {
	return func(c *FilterConfig) {
		c.IncludeIDs = append(c.IncludeIDs, ids...)
	}
}

// ExcludeTags filters controls to exclude those with any of the specified tags.
func ExcludeTags(tags ...string) FilterOption {
	return func(c *FilterConfig) {
		c.ExcludeTags = append(c.ExcludeTags, tags...)
	}
}

// ExcludeIDs filters controls to exclude those with the specified IDs.
func ExcludeIDs(ids ...string) FilterOption {
	return func(c *FilterConfig) {
		c.ExcludeIDs = append(c.ExcludeIDs, ids...)
	}
}

// Select returns a subset of controls that match the filtering criteria.
// Filters are applied as an intersection (AND) of valid conditions.
// Within a condition (e.g., Tags), it's a union (OR).
// If "Include" filters are empty, they are ignored (match all).
func (cs ControlSet) Select(opts ...FilterOption) ControlSet {
	config := &FilterConfig{}
	for _, opt := range opts {
		opt(config)
	}

	// Pre-process for faster lookup
	excludeIDMap := make(map[string]bool)
	for _, id := range config.ExcludeIDs {
		excludeIDMap[id] = true
	}

	includeIDMap := make(map[string]bool)
	for _, id := range config.IncludeIDs {
		includeIDMap[id] = true
	}

	excludeTagMap := make(map[string]bool)
	for _, tag := range config.ExcludeTags {
		excludeTagMap[tag] = true
	}

	var selected ControlSet
	for _, ctrl := range cs {
		// 1. Check Exclusions (Priority)
		if excludeIDMap[ctrl.ID] {
			continue
		}
		if len(excludeTagMap) > 0 && ctrl.HasAnyTag(config.ExcludeTags) {
			continue
		}

		// 2. Check Inclusions
		// ID Match
		if len(includeIDMap) > 0 && !includeIDMap[ctrl.ID] {
			// If IDs are specified, we generally expect *only* these IDs.
			// However, if we also have tags, standard behavior is usually AND.
			// But for ID selection, usually it means "these specific controls".
			// Let's stick to standard AND: if IDs list is present, ctrl.ID MUST be in it.
			continue
		}

		// Tag Match
		if len(config.IncludeTags) > 0 && !ctrl.HasAnyTag(config.IncludeTags) {
			continue
		}

		// Severity Match
		if len(config.IncludeSeverities) > 0 && !ctrl.MatchesAnySeverity(config.IncludeSeverities) {
			continue
		}

		selected = append(selected, ctrl)
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
