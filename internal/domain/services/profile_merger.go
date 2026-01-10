// Package services contains domain services for the Reglet domain model.
package services

import (
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// ProfileMerger merges multiple profiles according to inheritance semantics.
// This is a DOMAIN SERVICE because merge semantics are business rules.
//
// Merge Semantics:
//   - Metadata: overlay wins, fallback to base if empty
//   - Vars: deep merge, overlay wins on conflict
//   - Plugins: concatenate and deduplicate (preserving order)
//   - Controls.Defaults: deep merge, overlay wins (tags concatenate)
//   - Controls.Items: merge by ID (same ID = replace, new ID = append)
//   - Extends: NOT propagated (already resolved)
//
// SOLID Compliance:
//   - S: Only handles merge logic, no I/O
//   - O: Merge strategies could be extended via options
//   - D: Operates on domain entities only
//
// Future Extension Point:
// The API is designed to allow adding MergeOptions in the future for
// partial control merging if needed:
//
//	type MergeOptions struct {
//	    ControlMergeMode string // "replace" (default) or "partial"
//	}
type ProfileMerger struct{}

// NewProfileMerger creates a new profile merger service.
func NewProfileMerger() *ProfileMerger {
	return &ProfileMerger{}
}

// MergeAll merges multiple parents then applies the current profile.
// Parents are merged left-to-right (later parents win on conflict).
// Returns a NEW profile (does not mutate inputs).
func (m *ProfileMerger) MergeAll(
	parents []*entities.Profile,
	current *entities.Profile,
) *entities.Profile {
	if len(parents) == 0 {
		return DeepCopyProfile(current)
	}

	// Merge parents left-to-right
	result := DeepCopyProfile(parents[0])
	for _, parent := range parents[1:] {
		result = m.mergeTwoProfiles(result, parent)
	}

	// Apply current profile (highest priority)
	result = m.mergeTwoProfiles(result, current)

	return result
}

// Merge combines two profiles with overlay winning on conflicts.
// Returns a NEW profile (does not mutate inputs).
func (m *ProfileMerger) Merge(
	base *entities.Profile,
	overlay *entities.Profile,
) *entities.Profile {
	return m.mergeTwoProfiles(DeepCopyProfile(base), overlay)
}

// mergeTwoProfiles merges overlay onto base (mutates base).
// This is an internal helper; public methods ensure deep copy first.
func (m *ProfileMerger) mergeTwoProfiles(
	base *entities.Profile,
	overlay *entities.Profile,
) *entities.Profile {
	merged := &entities.Profile{}

	// Metadata: overlay wins, fallback to base if empty
	merged.Metadata = m.mergeMetadata(base.Metadata, overlay.Metadata)

	// Extends: NOT propagated (already resolved by loader)
	merged.Extends = nil

	// Vars: deep merge (overlay wins on conflict)
	merged.Vars = m.mergeVars(base.Vars, overlay.Vars)

	// Plugins: concatenate and deduplicate
	merged.Plugins = m.mergeStringSliceDedup(base.Plugins, overlay.Plugins)

	// Controls.Defaults: deep merge, overlay wins (tags concatenate)
	merged.Controls.Defaults = m.mergeDefaults(
		base.Controls.Defaults,
		overlay.Controls.Defaults,
	)

	// Controls.Items: merge by ID (same ID = replace, new ID = append)
	merged.Controls.Items = m.mergeControlItems(
		base.Controls.Items,
		overlay.Controls.Items,
	)

	return merged
}

// mergeMetadata merges profile metadata with overlay winning on non-empty fields.
func (m *ProfileMerger) mergeMetadata(
	base, overlay entities.ProfileMetadata,
) entities.ProfileMetadata {
	result := overlay
	if result.Name == "" {
		result.Name = base.Name
	}
	if result.Version == "" {
		result.Version = base.Version
	}
	if result.Description == "" {
		result.Description = base.Description
	}
	return result
}

// mergeVars performs a shallow merge of vars maps with overlay winning.
func (m *ProfileMerger) mergeVars(
	base, overlay map[string]interface{},
) map[string]interface{} {
	if base == nil && overlay == nil {
		return nil
	}
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v // Overlay wins on conflict
	}
	return result
}

// mergeStringSliceDedup concatenates two slices and deduplicates, preserving order.
func (m *ProfileMerger) mergeStringSliceDedup(base, overlay []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(base)+len(overlay))
	for _, s := range base {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range overlay {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// mergeDefaults merges control defaults with overlay winning, but tags concatenate.
func (m *ProfileMerger) mergeDefaults(
	base, overlay *entities.ControlDefaults,
) *entities.ControlDefaults {
	if base == nil && overlay == nil {
		return nil
	}
	result := &entities.ControlDefaults{}
	if base != nil {
		result.Severity = base.Severity
		result.Owner = base.Owner
		result.Tags = CopyStringSlice(base.Tags)
		result.Timeout = base.Timeout
	}
	if overlay != nil {
		if overlay.Severity != "" {
			result.Severity = overlay.Severity
		}
		if overlay.Owner != "" {
			result.Owner = overlay.Owner
		}
		// Tags: concatenate and deduplicate (not replace)
		result.Tags = m.mergeStringSliceDedup(result.Tags, overlay.Tags)
		if overlay.Timeout > 0 {
			result.Timeout = overlay.Timeout
		}
	}
	return result
}

// mergeControlItems merges controls by ID.
// Same ID = overlay replaces base (full control replacement).
// New ID = appended to result.
// Order is preserved: base controls first, then new overlay controls.
func (m *ProfileMerger) mergeControlItems(
	base, overlay []entities.Control,
) []entities.Control {
	// Build a map of overlay controls for O(1) lookup
	overlayMap := make(map[string]entities.Control)
	overlayOrder := make([]string, 0, len(overlay))
	for _, ctrl := range overlay {
		overlayMap[ctrl.ID] = ctrl
		overlayOrder = append(overlayOrder, ctrl.ID)
	}

	// Track which IDs we've seen to preserve order
	seenIDs := make(map[string]bool)
	result := make([]entities.Control, 0, len(base)+len(overlay))

	// First, process base controls (replace if overlay has same ID)
	for _, baseCtrl := range base {
		seenIDs[baseCtrl.ID] = true
		if overlayCtrl, exists := overlayMap[baseCtrl.ID]; exists {
			// Overlay replaces base (full replacement, not partial merge)
			result = append(result, CopyControls([]entities.Control{overlayCtrl})[0])
		} else {
			// Keep base control
			result = append(result, CopyControls([]entities.Control{baseCtrl})[0])
		}
	}

	// Then, append new controls from overlay (not in base)
	for _, id := range overlayOrder {
		if !seenIDs[id] {
			result = append(result, CopyControls([]entities.Control{overlayMap[id]})[0])
		}
	}

	return result
}
