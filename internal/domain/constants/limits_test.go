package constants_test

import (
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/constants"
)

// TestDefaultsLessThanAbsoluteMaximums verifies all defaults are within absolute bounds.
func TestDefaultsLessThanAbsoluteMaximums(t *testing.T) {
	tests := []struct {
		name     string
		default_ int
		absolute int
	}{
		{
			name:     "CommandOutputSize",
			default_: constants.DefaultMaxCommandOutputSize,
			absolute: constants.AbsoluteMaxCommandOutputSize,
		},
		{
			name:     "HTTPResponseSize",
			default_: constants.DefaultMaxHTTPResponseSize,
			absolute: constants.AbsoluteMaxHTTPResponseSize,
		},
		{
			name:     "EvidenceSize",
			default_: constants.DefaultMaxEvidenceSize,
			absolute: constants.AbsoluteMaxEvidenceSize,
		},
		{
			name:     "SARIFArtifactSize",
			default_: constants.DefaultMaxSARIFArtifactSize,
			absolute: constants.AbsoluteMaxSARIFArtifactSize,
		},
		{
			name:     "ExpressionLength",
			default_: constants.DefaultMaxExpressionLength,
			absolute: constants.AbsoluteMaxExpressionLength,
		},
		{
			name:     "ASTNodes",
			default_: constants.DefaultMaxASTNodes,
			absolute: constants.AbsoluteMaxASTNodes,
		},
		{
			name:     "HTTPRedirects",
			default_: constants.DefaultMaxHTTPRedirects,
			absolute: constants.AbsoluteMaxHTTPRedirects,
		},
		{
			name:     "ConcurrentControls",
			default_: constants.DefaultMinConcurrentControls, // Compare min to absolute max
			absolute: constants.AbsoluteMaxConcurrentControls,
		},
		{
			name:     "ConcurrentObservations",
			default_: constants.DefaultMaxConcurrentObservations,
			absolute: constants.AbsoluteMaxConcurrentObservations,
		},
		{
			name:     "WasmMemoryLimit",
			default_: constants.DefaultWasmMemoryLimitMB,
			absolute: constants.AbsoluteMaxWasmMemoryLimitMB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.default_ > tt.absolute {
				t.Errorf("Default %d exceeds absolute maximum %d", tt.default_, tt.absolute)
			}
			if tt.default_ <= 0 {
				t.Errorf("Default must be positive, got %d", tt.default_)
			}
			if tt.absolute <= 0 {
				t.Errorf("Absolute maximum must be positive, got %d", tt.absolute)
			}
		})
	}
}

// TestTimeoutDefaultsLessThanMaximums verifies timeout defaults are within bounds.
func TestTimeoutDefaultsLessThanMaximums(t *testing.T) {
	tests := []struct {
		name     string
		default_ time.Duration
		absolute time.Duration
	}{
		{
			name:     "HTTPTimeout",
			default_: constants.DefaultHTTPTimeout,
			absolute: constants.AbsoluteMaxHTTPTimeout,
		},
		{
			name:     "HTTPIdleTimeout",
			default_: constants.DefaultHTTPIdleTimeout,
			absolute: constants.AbsoluteMaxHTTPIdleTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.default_ > tt.absolute {
				t.Errorf("Default %v exceeds absolute maximum %v", tt.default_, tt.absolute)
			}
			if tt.default_ <= 0 {
				t.Errorf("Default must be positive, got %v", tt.default_)
			}
			if tt.absolute <= 0 {
				t.Errorf("Absolute maximum must be positive, got %v", tt.absolute)
			}
		})
	}
}

// TestSizeCalculationsNoOverflow verifies size multiplications don't overflow.
func TestSizeCalculationsNoOverflow(t *testing.T) {
	// Test that common size calculations won't overflow int
	sizes := []struct {
		name  string
		value int
	}{
		{"MaxRequestSize", constants.MaxRequestSize},
		{"DefaultMaxCommandOutputSize", constants.DefaultMaxCommandOutputSize},
		{"AbsoluteMaxCommandOutputSize", constants.AbsoluteMaxCommandOutputSize},
		{"DefaultMaxHTTPResponseSize", constants.DefaultMaxHTTPResponseSize},
		{"AbsoluteMaxHTTPResponseSize", constants.AbsoluteMaxHTTPResponseSize},
		{"DefaultMaxEvidenceSize", constants.DefaultMaxEvidenceSize},
		{"AbsoluteMaxEvidenceSize", constants.AbsoluteMaxEvidenceSize},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			// Verify value fits in int (no overflow in calculation)
			if sz.value < 0 {
				t.Errorf("%s resulted in negative value (overflow): %d", sz.name, sz.value)
			}

			// Verify reasonable bounds (< 1GB for safety checks)
			const oneGB = 1024 * 1024 * 1024
			if sz.value > oneGB {
				t.Errorf("%s exceeds 1GB: %d bytes", sz.name, sz.value)
			}
		})
	}
}

// TestConcurrencyDefaultsReasonable verifies concurrency limits are practical.
func TestConcurrencyDefaultsReasonable(t *testing.T) {
	// Min controls should be small but > 0
	if constants.DefaultMinConcurrentControls < 1 {
		t.Errorf("DefaultMinConcurrentControls too small: %d", constants.DefaultMinConcurrentControls)
	}
	if constants.DefaultMinConcurrentControls > 100 {
		t.Errorf("DefaultMinConcurrentControls unnecessarily large: %d", constants.DefaultMinConcurrentControls)
	}

	// Max observations should be reasonable
	if constants.DefaultMaxConcurrentObservations < 1 {
		t.Errorf("DefaultMaxConcurrentObservations too small: %d", constants.DefaultMaxConcurrentObservations)
	}
	if constants.DefaultMaxConcurrentObservations > 100 {
		t.Errorf("DefaultMaxConcurrentObservations unnecessarily large: %d", constants.DefaultMaxConcurrentObservations)
	}

	// Absolute maximums should prevent runaway concurrency
	if constants.AbsoluteMaxConcurrentControls > 10000 {
		t.Errorf("AbsoluteMaxConcurrentControls too large (DoS risk): %d", constants.AbsoluteMaxConcurrentControls)
	}
	if constants.AbsoluteMaxConcurrentObservations > 1000 {
		t.Errorf("AbsoluteMaxConcurrentObservations too large (DoS risk): %d", constants.AbsoluteMaxConcurrentObservations)
	}
}

// TestNonConfigurableLimitsAreReasonable verifies non-configurable security limits.
func TestNonConfigurableLimitsAreReasonable(t *testing.T) {
	// MaxRequestSize should be small enough to prevent DoS but large enough for realistic requests
	if constants.MaxRequestSize < 1024 {
		t.Errorf("MaxRequestSize too small for realistic use: %d bytes", constants.MaxRequestSize)
	}
	if constants.MaxRequestSize > 10*1024*1024 {
		t.Errorf("MaxRequestSize too large (DoS risk): %d bytes", constants.MaxRequestSize)
	}
}

// TestWasmMemoryLimitsReasonable verifies WASM memory limits are practical.
func TestWasmMemoryLimitsReasonable(t *testing.T) {
	// Default should be enough for typical plugins
	if constants.DefaultWasmMemoryLimitMB < 64 {
		t.Errorf("DefaultWasmMemoryLimitMB too small: %d MB", constants.DefaultWasmMemoryLimitMB)
	}

	// Absolute max should prevent excessive memory usage
	if constants.AbsoluteMaxWasmMemoryLimitMB > 8192 {
		t.Errorf("AbsoluteMaxWasmMemoryLimitMB too large: %d MB", constants.AbsoluteMaxWasmMemoryLimitMB)
	}

	// Default <= Absolute
	if constants.DefaultWasmMemoryLimitMB > constants.AbsoluteMaxWasmMemoryLimitMB {
		t.Errorf("Default WASM memory %d exceeds absolute max %d",
			constants.DefaultWasmMemoryLimitMB, constants.AbsoluteMaxWasmMemoryLimitMB)
	}
}
