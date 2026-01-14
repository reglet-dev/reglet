package system

import (
	"fmt"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/constants"
)

// LimitsConfig defines configurable limits for security, performance, and resource usage.
// This can be specified at system level (~/.reglet/config.yaml) or profile level.
// Profile limits override system limits, but neither can exceed absolute maximums defined in constants.
//
// All fields are pointers to distinguish between "not set" (nil) and "set to value".
// This allows proper merging: profile can override system, system can override defaults.
type LimitsConfig struct {
	// Evidence & Data Limits
	MaxEvidenceSize      *int `yaml:"max_evidence_size,omitempty"`
	MaxHTTPResponseSize  *int `yaml:"max_http_response_size,omitempty"`
	MaxCommandOutputSize *int `yaml:"max_command_output_size,omitempty"`
	MaxSARIFArtifactSize *int `yaml:"max_sarif_artifact_size,omitempty"`

	// Expression Evaluation Limits
	MaxExpressionLength *int `yaml:"max_expression_length,omitempty"`
	MaxASTNodes         *int `yaml:"max_ast_nodes,omitempty"`

	// Network & HTTP Limits
	MaxHTTPRedirects *int           `yaml:"max_http_redirects,omitempty"`
	HTTPTimeout      *time.Duration `yaml:"http_timeout,omitempty"`
	HTTPIdleTimeout  *time.Duration `yaml:"http_idle_timeout,omitempty"`

	// Concurrency Limits
	MaxConcurrentControls     *int `yaml:"max_concurrent_controls,omitempty"`
	MaxConcurrentObservations *int `yaml:"max_concurrent_observations,omitempty"`
}

// Validate checks that all configured limits are within acceptable bounds.
// Returns a detailed error if any limit exceeds its absolute maximum.
func (l *LimitsConfig) Validate() error {
	if l == nil {
		return nil
	}

	// Helper to check int limits
	checkIntLimit := func(name string, value *int, absoluteMax int) error {
		if value == nil {
			return nil
		}
		if *value < 0 {
			return fmt.Errorf("invalid limit %s: value must be non-negative, got %d", name, *value)
		}
		if *value > absoluteMax {
			return fmt.Errorf("invalid limit %s: configured value %d exceeds absolute maximum %d (security boundary)",
				name, *value, absoluteMax)
		}
		return nil
	}

	// Helper to check duration limits
	checkDurationLimit := func(name string, value *time.Duration, absoluteMax time.Duration) error {
		if value == nil {
			return nil
		}
		if *value < 0 {
			return fmt.Errorf("invalid limit %s: value must be non-negative, got %v", name, *value)
		}
		if *value > absoluteMax {
			return fmt.Errorf("invalid limit %s: configured value %v exceeds absolute maximum %v (security boundary)",
				name, *value, absoluteMax)
		}
		return nil
	}

	// Validate Evidence & Data limits
	if err := checkIntLimit("max_evidence_size", l.MaxEvidenceSize, constants.AbsoluteMaxEvidenceSize); err != nil {
		return err
	}
	if err := checkIntLimit("max_http_response_size", l.MaxHTTPResponseSize, constants.AbsoluteMaxHTTPResponseSize); err != nil {
		return err
	}
	if err := checkIntLimit("max_command_output_size", l.MaxCommandOutputSize, constants.AbsoluteMaxCommandOutputSize); err != nil {
		return err
	}
	if err := checkIntLimit("max_sarif_artifact_size", l.MaxSARIFArtifactSize, constants.AbsoluteMaxSARIFArtifactSize); err != nil {
		return err
	}

	// Validate Expression limits
	if err := checkIntLimit("max_expression_length", l.MaxExpressionLength, constants.AbsoluteMaxExpressionLength); err != nil {
		return err
	}
	if err := checkIntLimit("max_ast_nodes", l.MaxASTNodes, constants.AbsoluteMaxASTNodes); err != nil {
		return err
	}

	// Validate Network & HTTP limits
	if err := checkIntLimit("max_http_redirects", l.MaxHTTPRedirects, constants.AbsoluteMaxHTTPRedirects); err != nil {
		return err
	}
	if err := checkDurationLimit("http_timeout", l.HTTPTimeout, constants.AbsoluteMaxHTTPTimeout); err != nil {
		return err
	}
	if err := checkDurationLimit("http_idle_timeout", l.HTTPIdleTimeout, constants.AbsoluteMaxHTTPIdleTimeout); err != nil {
		return err
	}

	// Validate Concurrency limits
	if err := checkIntLimit("max_concurrent_controls", l.MaxConcurrentControls, constants.AbsoluteMaxConcurrentControls); err != nil {
		return err
	}
	return checkIntLimit("max_concurrent_observations", l.MaxConcurrentObservations, constants.AbsoluteMaxConcurrentObservations)
}

// Merge applies overrides from another LimitsConfig.
// Non-nil values in override take precedence over values in l.
// Returns a new LimitsConfig with merged values (does not modify l or override).
func (l *LimitsConfig) Merge(override *LimitsConfig) *LimitsConfig {
	if override == nil {
		// Nothing to merge, return copy of current
		return l.copy()
	}
	if l == nil {
		// Base is nil, return copy of override
		return override.copy()
	}

	// Merge: override wins when non-nil
	result := &LimitsConfig{}

	// Evidence & Data
	result.MaxEvidenceSize = mergeIntPtr(l.MaxEvidenceSize, override.MaxEvidenceSize)
	result.MaxHTTPResponseSize = mergeIntPtr(l.MaxHTTPResponseSize, override.MaxHTTPResponseSize)
	result.MaxCommandOutputSize = mergeIntPtr(l.MaxCommandOutputSize, override.MaxCommandOutputSize)
	result.MaxSARIFArtifactSize = mergeIntPtr(l.MaxSARIFArtifactSize, override.MaxSARIFArtifactSize)

	// Expression Evaluation
	result.MaxExpressionLength = mergeIntPtr(l.MaxExpressionLength, override.MaxExpressionLength)
	result.MaxASTNodes = mergeIntPtr(l.MaxASTNodes, override.MaxASTNodes)

	// Network & HTTP
	result.MaxHTTPRedirects = mergeIntPtr(l.MaxHTTPRedirects, override.MaxHTTPRedirects)
	result.HTTPTimeout = mergeDurationPtr(l.HTTPTimeout, override.HTTPTimeout)
	result.HTTPIdleTimeout = mergeDurationPtr(l.HTTPIdleTimeout, override.HTTPIdleTimeout)

	// Concurrency
	result.MaxConcurrentControls = mergeIntPtr(l.MaxConcurrentControls, override.MaxConcurrentControls)
	result.MaxConcurrentObservations = mergeIntPtr(l.MaxConcurrentObservations, override.MaxConcurrentObservations)

	return result
}

// copy creates a deep copy of the LimitsConfig.
func (l *LimitsConfig) copy() *LimitsConfig {
	if l == nil {
		return nil
	}

	return &LimitsConfig{
		MaxEvidenceSize:           copyIntPtr(l.MaxEvidenceSize),
		MaxHTTPResponseSize:       copyIntPtr(l.MaxHTTPResponseSize),
		MaxCommandOutputSize:      copyIntPtr(l.MaxCommandOutputSize),
		MaxSARIFArtifactSize:      copyIntPtr(l.MaxSARIFArtifactSize),
		MaxExpressionLength:       copyIntPtr(l.MaxExpressionLength),
		MaxASTNodes:               copyIntPtr(l.MaxASTNodes),
		MaxHTTPRedirects:          copyIntPtr(l.MaxHTTPRedirects),
		HTTPTimeout:               copyDurationPtr(l.HTTPTimeout),
		HTTPIdleTimeout:           copyDurationPtr(l.HTTPIdleTimeout),
		MaxConcurrentControls:     copyIntPtr(l.MaxConcurrentControls),
		MaxConcurrentObservations: copyIntPtr(l.MaxConcurrentObservations),
	}
}

// Helper functions for merging pointer values

func mergeIntPtr(base, override *int) *int {
	if override != nil {
		return copyIntPtr(override)
	}
	return copyIntPtr(base)
}

func mergeDurationPtr(base, override *time.Duration) *time.Duration {
	if override != nil {
		return copyDurationPtr(override)
	}
	return copyDurationPtr(base)
}

func copyIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func copyDurationPtr(v *time.Duration) *time.Duration {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}
