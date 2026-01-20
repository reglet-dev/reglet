package ports

import (
	"context"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	ProfileSpecific *capabilities.Capability
	Capability      capabilities.Capability
	PluginName      string
	IsProfileBased  bool
	IsBroad         bool
}

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime PluginRuntime, pluginDir string) (map[string][]capabilities.Capability, error)
}

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This allows the orchestrator to be tested with mock analyzers.
type CapabilityAnalyzer interface {
	ExtractCapabilities(profile entities.ProfileReader) map[string][]capabilities.Capability
}

// CapabilityGatekeeperPort grants capabilities based on security policy.
// Named with "Port" suffix to avoid collision with the concrete CapabilityGatekeeper type.
type CapabilityGatekeeperPort interface {
	GrantCapabilities(
		required capabilities.Grant,
		capabilityInfo map[string]CapabilityInfo,
		trustAll bool,
	) (capabilities.Grant, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(ctx context.Context, required map[string][]capabilities.Capability, trustAll bool) (map[string][]capabilities.Capability, error)
}

// DataRedactor scrubs sensitive information from output.
// Placed here as it's often used with execution/security components.
type DataRedactor interface {
	Redact(input string) string
}

// IntegrityVerifier verifies cryptographic signatures on plugin artifacts.
type IntegrityVerifier interface {
	// VerifySignature checks the signature of a plugin in the registry.
	VerifySignature(ctx context.Context, ref values.PluginReference) (*SignatureResult, error)

	// Sign signs a plugin artifact (for publishing).
	Sign(ctx context.Context, ref values.PluginReference) error
}

// SignatureResult contains signature verification details.
type SignatureResult struct {
	SignedAt        time.Time
	Signer          string
	TransparencyLog string
	Certificate     []byte
	Verified        bool
}

// ProfileTrustChecker verifies trust for remote profile sources.
type ProfileTrustChecker interface {
	// RequiresTrust returns true if the profile path requires trust verification.
	RequiresTrust(path string) bool

	// IsTrusted returns true if the URL matches a trusted source pattern.
	IsTrusted(url string) bool

	// PromptForTrust prompts the user to trust a remote profile.
	// Returns true if trusted, false if denied, or error for non-interactive mode.
	PromptForTrust(
		ctx context.Context,
		url string,
		requiredCaps map[string][]capabilities.Capability,
		trustFlag bool,
	) (bool, error)
}
