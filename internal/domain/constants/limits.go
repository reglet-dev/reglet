// Package constants defines application-wide constants and limits.
// This package provides centralized definitions for all configurable limits,
// including both default values and absolute security maximums.
package constants

import "time"

// ════════════════════════════════════════════════════════════════════════════
// Security Limits - Request/Response Size Constraints
// ════════════════════════════════════════════════════════════════════════════

// MaxRequestSize is the absolute maximum size for WASM guest memory requests (1MB).
// This is NOT configurable as it's a critical security boundary preventing
// malicious/buggy WASM modules from triggering OOM via excessive memory allocation.
// This limit protects the host system from DoS attacks.
const MaxRequestSize = 1 * 1024 * 1024

// DefaultMaxCommandOutputSize is the default limit for stdout/stderr from exec commands (10MB).
// Prevents excessive memory usage from long-running commands with verbose output.
const DefaultMaxCommandOutputSize = 10 * 1024 * 1024

// AbsoluteMaxCommandOutputSize is the absolute maximum for command output (100MB).
// Even with configuration, command output cannot exceed this to prevent OOM.
const AbsoluteMaxCommandOutputSize = 100 * 1024 * 1024

// DefaultMaxHTTPResponseSize is the default limit for HTTP response bodies (10MB).
// Protects against downloading excessively large files via HTTP plugin.
const DefaultMaxHTTPResponseSize = 10 * 1024 * 1024

// AbsoluteMaxHTTPResponseSize is the absolute maximum for HTTP responses (100MB).
// Hard limit even when configured higher to prevent memory exhaustion.
const AbsoluteMaxHTTPResponseSize = 100 * 1024 * 1024

// ════════════════════════════════════════════════════════════════════════════
// Evidence Limits - OSCAL Evidence Collection
// ════════════════════════════════════════════════════════════════════════════

// DefaultMaxEvidenceSize is the default limit for observation evidence data (1MB).
// Evidence larger than this will be truncated with metadata preserved.
const DefaultMaxEvidenceSize = 1 * 1024 * 1024

// AbsoluteMaxEvidenceSize is the absolute maximum for evidence size (10MB).
// Prevents profiles from requesting unbounded evidence storage.
const AbsoluteMaxEvidenceSize = 10 * 1024 * 1024

// DefaultMaxSARIFArtifactSize is the default limit for embedded file content in SARIF (512KB).
// SARIF outputs can embed source files; this limits file size to keep reports manageable.
const DefaultMaxSARIFArtifactSize = 512 * 1024

// AbsoluteMaxSARIFArtifactSize is the absolute maximum for SARIF artifacts (5MB).
// Prevents SARIF files from becoming excessively large.
const AbsoluteMaxSARIFArtifactSize = 5 * 1024 * 1024

// ════════════════════════════════════════════════════════════════════════════
// Expression Evaluation Limits - DoS Prevention
// ════════════════════════════════════════════════════════════════════════════

// DefaultMaxExpressionLength is the default maximum length for expect expressions (1000 chars).
// Long expressions are hard to read and can hide complexity; this encourages clarity.
const DefaultMaxExpressionLength = 1000

// AbsoluteMaxExpressionLength is the absolute maximum expression length (10000 chars).
// Prevents DoS via extremely long expression strings.
const AbsoluteMaxExpressionLength = 10000

// DefaultMaxASTNodes is the default maximum AST nodes in an expression (100).
// Limits computational complexity of expression evaluation to prevent DoS
// via deeply nested or complex expressions (e.g., repeated parentheses).
const DefaultMaxASTNodes = 100

// AbsoluteMaxASTNodes is the absolute maximum AST nodes (1000).
// Hard cap on expression complexity even when configured higher.
const AbsoluteMaxASTNodes = 1000

// ════════════════════════════════════════════════════════════════════════════
// Network & HTTP Limits
// ════════════════════════════════════════════════════════════════════════════

// DefaultMaxHTTPRedirects is the default maximum HTTP redirect chain length (10).
// Prevents infinite redirect loops and excessive request chains.
const DefaultMaxHTTPRedirects = 10

// AbsoluteMaxHTTPRedirects is the absolute maximum redirect chain length (50).
// Hard cap on redirects to prevent abuse.
const AbsoluteMaxHTTPRedirects = 50

// DefaultHTTPTimeout is the default timeout for HTTP requests (30 seconds).
// Balances between slow endpoints and preventing hung requests.
const DefaultHTTPTimeout = 30 * time.Second

// AbsoluteMaxHTTPTimeout is the absolute maximum HTTP timeout (5 minutes).
// Prevents profiles from setting unreasonably long timeouts.
const AbsoluteMaxHTTPTimeout = 5 * time.Minute

// DefaultHTTPIdleTimeout is the default idle connection timeout (90 seconds).
// How long to keep idle HTTP connections alive for reuse.
const DefaultHTTPIdleTimeout = 90 * time.Second

// AbsoluteMaxHTTPIdleTimeout is the absolute maximum idle timeout (10 minutes).
const AbsoluteMaxHTTPIdleTimeout = 10 * time.Minute

// DefaultHTTPTLSHandshakeTimeout is the default TLS handshake timeout (10 seconds).
const DefaultHTTPTLSHandshakeTimeout = 10 * time.Second

// DefaultHTTPExpectContinueTimeout is the default Expect: 100-continue timeout (1 second).
const DefaultHTTPExpectContinueTimeout = 1 * time.Second

// ════════════════════════════════════════════════════════════════════════════
// Concurrency Limits
// ════════════════════════════════════════════════════════════════════════════

// DefaultMinConcurrentControls is the minimum concurrent control executions (4).
// Ensures reasonable parallelism even on single-core systems.
const DefaultMinConcurrentControls = 4

// DefaultMaxConcurrentObservations is the maximum concurrent observations per control (10).
// Caps nested parallelism to avoid excessive goroutine creation.
const DefaultMaxConcurrentObservations = 10

// DefaultMinConcurrentObservations is the minimum concurrent observations (2).
// Ensures some parallelism for observations within a control.
const DefaultMinConcurrentObservations = 2

// AbsoluteMaxConcurrentControls is the absolute maximum concurrent controls (1000).
// Prevents unreasonable concurrency settings that could exhaust resources.
const AbsoluteMaxConcurrentControls = 1000

// AbsoluteMaxConcurrentObservations is the absolute maximum concurrent observations (100).
const AbsoluteMaxConcurrentObservations = 100

// ════════════════════════════════════════════════════════════════════════════
// WASM Runtime Limits
// ════════════════════════════════════════════════════════════════════════════

// DefaultWasmMemoryLimitMB is the default WASM instance memory limit (512MB).
// Each WASM plugin instance gets this much memory allocation.
const DefaultWasmMemoryLimitMB = 512

// AbsoluteMaxWasmMemoryLimitMB is the absolute maximum WASM memory (4096MB = 4GB).
// Prevents profiles from requesting excessive WASM memory.
// Note: This is separate from MaxRequestSize which limits individual allocations.
const AbsoluteMaxWasmMemoryLimitMB = 4096
