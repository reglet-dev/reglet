package engine

import (
	"fmt"

	"github.com/whiskeyjimbo/reglet/internal/config"
)

// ControlLevel represents a batch of controls that can run in parallel.
// Level 0 = no dependencies, Level 1 = depends on Level 0, etc.
type ControlLevel struct {
	Level    int
	Controls []config.Control
}

// BuildControlDAG builds a dependency graph and returns controls grouped by level.
// Uses Kahn's algorithm for topological sorting.
//
// Returns:
//   - []ControlLevel: Controls grouped by dependency level (Level 0, 1, 2, etc.)
//   - error: If circular dependency detected or dependency references non-existent control
func BuildControlDAG(controls []config.Control) ([]ControlLevel, error) {
	// Build adjacency map
	controlMap := make(map[string]config.Control)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	// Initialize all controls
	for _, ctrl := range controls {
		controlMap[ctrl.ID] = ctrl
		inDegree[ctrl.ID] = 0
	}

	// Build dependency graph
	for _, ctrl := range controls {
		for _, depID := range ctrl.DependsOn {
			// Validate dependency exists
			if _, exists := controlMap[depID]; !exists {
				return nil, fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, depID)
			}

			// Increment in-degree for this control (it has one more dependency)
			inDegree[ctrl.ID]++

			// Track dependents (controls that depend on depID)
			dependents[depID] = append(dependents[depID], ctrl.ID)
		}
	}

	// Topological sort using Kahn's algorithm
	var levels []ControlLevel
	currentLevel := 0
	processed := make(map[string]bool)

	for len(processed) < len(controls) {
		// Find all controls with inDegree == 0 (no unmet dependencies)
		var levelControls []config.Control

		for _, ctrl := range controls {
			if processed[ctrl.ID] {
				continue
			}

			if inDegree[ctrl.ID] == 0 {
				levelControls = append(levelControls, ctrl)
			}
		}

		if len(levelControls) == 0 {
			// Cycle detected! No controls have inDegree 0, but not all processed
			var unprocessed []string
			for _, ctrl := range controls {
				if !processed[ctrl.ID] {
					unprocessed = append(unprocessed, ctrl.ID)
				}
			}
			return nil, fmt.Errorf("circular dependency detected among controls: %v", unprocessed)
		}

		// Add this level
		levels = append(levels, ControlLevel{
			Level:    currentLevel,
			Controls: levelControls,
		})

		// Process level: mark as done and decrement dependents' in-degree
		for _, ctrl := range levelControls {
			processed[ctrl.ID] = true

			// Decrement in-degree for all controls that depend on this one
			for _, depID := range dependents[ctrl.ID] {
				inDegree[depID]--
			}
		}

		currentLevel++
	}

	return levels, nil
}
