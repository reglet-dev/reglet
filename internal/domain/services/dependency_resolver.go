package services

import (
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// DependencyResolver handles control dependency graph operations
type DependencyResolver struct{}

// NewDependencyResolver creates a new dependency resolver service
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{}
}

// ControlLevel represents controls at a specific dependency level
type ControlLevel struct {
	Level    int
	Controls []entities.Control
}

// BuildControlDAG builds a dependency graph using Kahn's algorithm.
// Returns controls grouped by level for parallel execution within levels.
//
// Algorithm:
// 1. Build adjacency list and in-degree map
// 2. Find all controls with no dependencies (in-degree 0)
// 3. Process controls level by level, decrementing in-degrees
// 4. Detect cycles (remaining controls with in-degree > 0)
func (r *DependencyResolver) BuildControlDAG(controls []entities.Control) ([]ControlLevel, error) {
	// Build maps for efficient lookup
	controlByID := make(map[string]entities.Control)
	inDegree := make(map[string]int)
	dependencies := make(map[string][]string) // reverse map: controlID -> dependents

	for _, ctrl := range controls {
		controlByID[ctrl.ID] = ctrl
		inDegree[ctrl.ID] = len(ctrl.DependsOn)

		// Build reverse dependency map
		for _, dep := range ctrl.DependsOn {
			dependencies[dep] = append(dependencies[dep], ctrl.ID)
		}
	}

	// Validate all dependencies exist
	for _, ctrl := range controls {
		for _, dep := range ctrl.DependsOn {
			if _, exists := controlByID[dep]; !exists {
				return nil, fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, dep)
			}
		}
	}

	var levels []ControlLevel
	processed := make(map[string]bool)
	level := 0

	for len(processed) < len(controls) {
		var currentLevel []entities.Control

		// Find all controls with in-degree 0 (no unmet dependencies)
		for _, ctrl := range controls {
			if processed[ctrl.ID] {
				continue
			}
			if inDegree[ctrl.ID] == 0 {
				currentLevel = append(currentLevel, ctrl)
			}
		}

		// No progress made → cycle detected
		if len(currentLevel) == 0 {
			remaining := []string{}
			for _, ctrl := range controls {
				if !processed[ctrl.ID] {
					remaining = append(remaining, ctrl.ID)
				}
			}
			return nil, fmt.Errorf("circular dependency detected among controls: %v", remaining)
		}

		// Add level to result
		levels = append(levels, ControlLevel{
			Level:    level,
			Controls: currentLevel,
		})

		// Mark controls as processed and decrement dependent in-degrees
		for _, ctrl := range currentLevel {
			processed[ctrl.ID] = true

			// Decrement in-degree for dependent controls
			for _, dependent := range dependencies[ctrl.ID] {
				inDegree[dependent]--
			}
		}

		level++
	}

	return levels, nil
}

// ResolveDependencies calculates transitive dependencies for each control.
// Returns map of controlID → set of all dependencies (direct + transitive).
//
// Used by --include-dependencies flag to include all controls in dependency chain.
func (r *DependencyResolver) ResolveDependencies(controls []entities.Control) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	controlByID := make(map[string]entities.Control)

	for _, ctrl := range controls {
		controlByID[ctrl.ID] = ctrl
		result[ctrl.ID] = make(map[string]bool)
	}

	// Validate all dependencies exist
	for _, ctrl := range controls {
		for _, dep := range ctrl.DependsOn {
			if _, exists := controlByID[dep]; !exists {
				return nil, fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, dep)
			}
		}
	}

	// Compute transitive closure using DFS
	var computeDeps func(controlID string, visited map[string]bool) error
	computeDeps = func(controlID string, visited map[string]bool) error {
		if visited[controlID] {
			return fmt.Errorf("circular dependency detected at control %s", controlID)
		}

		ctrl, exists := controlByID[controlID]
		if !exists {
			return fmt.Errorf("control %s not found", controlID)
		}

		visited[controlID] = true
		defer func() { visited[controlID] = false }()

		for _, dep := range ctrl.DependsOn {
			// Add direct dependency
			result[controlID][dep] = true

			// Recursively add transitive dependencies
			if err := computeDeps(dep, visited); err != nil {
				return err
			}

			// Add all dependencies of the dependency
			for transDep := range result[dep] {
				result[controlID][transDep] = true
			}
		}

		return nil
	}

	// Compute dependencies for each control
	for _, ctrl := range controls {
		if err := computeDeps(ctrl.ID, make(map[string]bool)); err != nil {
			return nil, err
		}
	}

	return result, nil
}
