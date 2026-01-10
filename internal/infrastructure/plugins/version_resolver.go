package plugins

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// SemverResolver implements ports.VersionResolver using Masterminds/semver.
type SemverResolver struct{}

// NewSemverResolver creates a new SemverResolver.
func NewSemverResolver() *SemverResolver {
	return &SemverResolver{}
}

// Resolve converts a version constraint to an exact version from the available options.
// It returns the highest version that satisfies the constraint.
func (r *SemverResolver) Resolve(constraint string, available []string) (string, error) {
	// 1. Parse constraint
	// If constraint is "latest", we treat it as ">= 0" but logically we Just want max.
	// Masterminds/semver doesn't handle "latest" keyword natively in constraints usually.
	var c *semver.Constraints
	var err error

	if constraint == "latest" {
		c, err = semver.NewConstraint(">= 0")
	} else {
		c, err = semver.NewConstraint(constraint)
	}

	if err != nil {
		return "", fmt.Errorf("invalid version constraint %q: %w", constraint, err)
	}

	// 2. Parse and filter available versions
	var valid []*semver.Version
	for _, vStr := range available {
		v, err := semver.NewVersion(vStr)
		if err != nil {
			continue // Skip invalid versions in availability list
		}

		// If constraint is satisfied
		if c.Check(v) {
			valid = append(valid, v)
		}
	}

	if len(valid) == 0 {
		return "", fmt.Errorf("no version satisfies constraint %q from available options", constraint)
	}

	// 3. Sort to find highest
	sort.Sort(semver.Collection(valid))

	// Collection sorts as 0.1, 0.2, 1.0 (ascending).
	// So last element is the highest.
	highest := valid[len(valid)-1]

	return highest.Original(), nil
}
