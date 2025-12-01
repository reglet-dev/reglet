package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadProfile loads and parses a profile from a YAML file.
// It applies control defaults and validates the profile structure.
func LoadProfile(path string) (*Profile, error) {
	// Security: Use os.OpenRoot to prevent path traversal attacks
	// resolving symlinks or escaping the intended directory.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile directory: %w", err)
	}
	defer root.Close()

	file, err := root.Open(base)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile: %w", err)
	}
	defer file.Close()

	return LoadProfileFromReader(file)
}

// LoadProfileFromReader loads and parses a profile from an io.Reader.
// This is useful for testing with in-memory YAML data.
func LoadProfileFromReader(r io.Reader) (*Profile, error) {
	var profile Profile

	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true) // Strict parsing - reject unknown fields

	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply control defaults to each control
	if err := applyDefaults(&profile); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	return &profile, nil
}

// applyDefaults applies control defaults to each control item.
// Individual control values take precedence over defaults.
func applyDefaults(profile *Profile) error {
	if profile.Controls.Defaults == nil {
		// No defaults to apply
		return nil
	}

	defaults := profile.Controls.Defaults

	for i := range profile.Controls.Items {
		ctrl := &profile.Controls.Items[i]

		// Apply severity if not set
		if ctrl.Severity == "" && defaults.Severity != "" {
			ctrl.Severity = defaults.Severity
		}

		// Apply owner if not set
		if ctrl.Owner == "" && defaults.Owner != "" {
			ctrl.Owner = defaults.Owner
		}

		// Apply timeout if not set
		if ctrl.Timeout == 0 && defaults.Timeout != 0 {
			ctrl.Timeout = defaults.Timeout
		}

		// Merge tags (defaults + control-specific)
		if len(defaults.Tags) > 0 {
			// Create a map to deduplicate tags
			tagMap := make(map[string]bool)

			// Add default tags
			for _, tag := range defaults.Tags {
				tagMap[tag] = true
			}

			// Add control-specific tags
			for _, tag := range ctrl.Tags {
				tagMap[tag] = true
			}

			// Convert back to slice
			mergedTags := make([]string, 0, len(tagMap))
			for tag := range tagMap {
				mergedTags = append(mergedTags, tag)
			}

			ctrl.Tags = mergedTags
		}
	}

	return nil
}
