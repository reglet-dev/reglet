package config

import (
	"runtime"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

// RuntimeConfig aggregates all runtime configuration.
// This is a value object that flows through the system.
type RuntimeConfig struct {
	// Security
	SecurityLevel string

	// Evidence
	MaxEvidenceSizeBytes int

	// WASM
	WasmMemoryLimitMB int

	// Concurrency
	MaxConcurrentControls     int
	MaxConcurrentObservations int
}

// FromSystemConfig creates RuntimeConfig from system config.
func FromSystemConfig(sys *system.Config) *RuntimeConfig {
	return &RuntimeConfig{
		MaxEvidenceSizeBytes: sys.MaxEvidenceSizeBytes,
		WasmMemoryLimitMB:    sys.WasmMemoryLimitMB,
		SecurityLevel:        string(sys.Security.GetSecurityLevel()),
	}
}

// ApplyDefaults applies defaults for zero values.
func (r *RuntimeConfig) ApplyDefaults() {
	if r.MaxEvidenceSizeBytes == 0 {
		r.MaxEvidenceSizeBytes = execution.DefaultMaxEvidenceSize
	}
	if r.WasmMemoryLimitMB == 0 {
		r.WasmMemoryLimitMB = 512 // Default 512MB per instance
	}
	if r.MaxConcurrentControls == 0 {
		r.MaxConcurrentControls = runtime.NumCPU()
	}
	// MaxConcurrentObservations defaults to 0 (unlimited) if not set, which is fine.
}
