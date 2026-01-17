package scaffold

import (
	"fmt"
	"regexp"
)

// profileNamePattern validates profile names: starts with letter, alphanumeric/hyphen/underscore, 1-64 chars.
var profileNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// InitOptions holds the configuration collected from the wizard or CLI flags.
// This value object encapsulates all user inputs needed for profile generation.
type InitOptions struct {
	// ProfileName is the name for the generated profile (required)
	ProfileName string

	// OutputPath is where to write the generated profile (default: ./reglet-profile.yaml)
	OutputPath string

	// Plugins is the list of selected plugins to include (required, 1-6 items)
	Plugins []string

	// WithConfig indicates whether to generate ~/.reglet/config.yaml
	WithConfig bool

	// Force allows overwriting existing files without prompting
	Force bool
}

// Validate checks that all required fields are present and valid.
// Returns an error describing the first validation failure, or nil if valid.
func (o *InitOptions) Validate() error {
	if err := o.validateProfileName(); err != nil {
		return err
	}
	if err := o.validatePlugins(); err != nil {
		return err
	}
	if o.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	return nil
}

// validateProfileName checks that the profile name matches the required pattern.
func (o *InitOptions) validateProfileName() error {
	if o.ProfileName == "" {
		return fmt.Errorf("profile name is required")
	}
	if !profileNamePattern.MatchString(o.ProfileName) {
		return fmt.Errorf("invalid profile name '%s': must start with a letter and contain only alphanumeric characters, hyphens, and underscores (max 64 chars)", o.ProfileName)
	}
	return nil
}

// validatePlugins checks that at least one valid plugin is selected.
func (o *InitOptions) validatePlugins() error {
	if len(o.Plugins) == 0 {
		return fmt.Errorf("at least one plugin must be selected")
	}

	validNames := ValidPluginNames()
	for _, plugin := range o.Plugins {
		if !validNames[plugin] {
			return fmt.Errorf("unknown plugin '%s': valid plugins are file, http, dns, tcp, command, smtp", plugin)
		}
	}
	return nil
}

// ValidateProfileName is a validation function for use with huh.Input.
// Returns an error if the name is invalid, nil otherwise.
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if !profileNamePattern.MatchString(name) {
		return fmt.Errorf("must start with letter, contain only alphanumeric, hyphen, underscore (max 64 chars)")
	}
	return nil
}

// DefaultOutputPath is the default output path for generated profiles.
const DefaultOutputPath = "./reglet-profile.yaml"

// DefaultConfigPath returns the default system config path.
// This is always ~/.reglet/config.yaml.
func DefaultConfigPath() string {
	return "~/.reglet/config.yaml"
}
