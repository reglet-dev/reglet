package config

import (
	"runtime"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/constants"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

// RuntimeConfig aggregates all runtime configuration.
// This is a value object that flows through the system.
type RuntimeConfig struct {
	Limits                    *ResolvedLimits
	SecurityLevel             string
	WasmMemoryLimitMB         int
	MaxEvidenceSizeBytes      int
	MaxConcurrentControls     int
	MaxConcurrentObservations int
}

// ResolvedLimits contains the final, resolved limit values after merging all sources.
// All fields are non-pointer primitive types for easy access throughout the codebase.
type ResolvedLimits struct {
	// Evidence & Data Limits
	MaxEvidenceSize      int
	MaxHTTPResponseSize  int
	MaxCommandOutputSize int
	MaxSARIFArtifactSize int

	// Expression Evaluation Limits
	MaxExpressionLength int
	MaxASTNodes         int

	// Network & HTTP Limits
	MaxHTTPRedirects int
	HTTPTimeout      time.Duration
	HTTPIdleTimeout  time.Duration

	// Concurrency Limits
	MaxConcurrentControls     int
	MaxConcurrentObservations int
}

// FromSystemConfig creates RuntimeConfig from system config.
// This is the legacy constructor for backward compatibility.
func FromSystemConfig(sys *system.Config) *RuntimeConfig {
	rc := &RuntimeConfig{
		MaxEvidenceSizeBytes: sys.MaxEvidenceSizeBytes,
		WasmMemoryLimitMB:    sys.WasmMemoryLimitMB,
		SecurityLevel:        string(sys.Security.GetSecurityLevel()),
	}

	// Build limits from system config only (no profile override)
	limits, _ := BuildLimits(sys.Limits, nil)
	rc.Limits = limits

	return rc
}

// FromSystemAndProfileConfig creates RuntimeConfig from both system and profile config.
// This merges limits with proper precedence: defaults → system → profile.
func FromSystemAndProfileConfig(sys *system.Config, profileLimits *system.LimitsConfig) (*RuntimeConfig, error) {
	rc := &RuntimeConfig{
		MaxEvidenceSizeBytes: sys.MaxEvidenceSizeBytes,
		WasmMemoryLimitMB:    sys.WasmMemoryLimitMB,
		SecurityLevel:        string(sys.Security.GetSecurityLevel()),
	}

	// Build limits with profile overrides
	limits, err := BuildLimits(sys.Limits, profileLimits)
	if err != nil {
		return nil, err
	}
	rc.Limits = limits

	return rc, nil
}

// BuildLimits merges limits from code defaults, system config, and profile config.
// Precedence: profile > system > defaults
// Validates all limits against absolute maximums.
func BuildLimits(systemLimits, profileLimits *system.LimitsConfig) (*ResolvedLimits, error) {
	// Handle nil systemLimits
	if systemLimits == nil {
		systemLimits = &system.LimitsConfig{}
	}

	// Merge system and profile limits (profile takes precedence)
	merged := systemLimits.Merge(profileLimits)

	// Validate the merged config
	if err := merged.Validate(); err != nil {
		return nil, err
	}

	// Build ResolvedLimits with defaults for any nil values
	return &ResolvedLimits{
		MaxEvidenceSize:           getIntOrDefault(merged.MaxEvidenceSize, constants.DefaultMaxEvidenceSize),
		MaxHTTPResponseSize:       getIntOrDefault(merged.MaxHTTPResponseSize, constants.DefaultMaxHTTPResponseSize),
		MaxCommandOutputSize:      getIntOrDefault(merged.MaxCommandOutputSize, constants.DefaultMaxCommandOutputSize),
		MaxSARIFArtifactSize:      getIntOrDefault(merged.MaxSARIFArtifactSize, constants.DefaultMaxSARIFArtifactSize),
		MaxExpressionLength:       getIntOrDefault(merged.MaxExpressionLength, constants.DefaultMaxExpressionLength),
		MaxASTNodes:               getIntOrDefault(merged.MaxASTNodes, constants.DefaultMaxASTNodes),
		MaxHTTPRedirects:          getIntOrDefault(merged.MaxHTTPRedirects, constants.DefaultMaxHTTPRedirects),
		HTTPTimeout:               getDurationOrDefault(merged.HTTPTimeout, constants.DefaultHTTPTimeout),
		HTTPIdleTimeout:           getDurationOrDefault(merged.HTTPIdleTimeout, constants.DefaultHTTPIdleTimeout),
		MaxConcurrentControls:     getIntOrDefault(merged.MaxConcurrentControls, runtime.NumCPU()),
		MaxConcurrentObservations: getIntOrDefault(merged.MaxConcurrentObservations, constants.DefaultMaxConcurrentObservations),
	}, nil
}

// Helper functions

func getIntOrDefault(ptr *int, defaultVal int) int {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

func getDurationOrDefault(ptr *time.Duration, defaultVal time.Duration) time.Duration {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}
