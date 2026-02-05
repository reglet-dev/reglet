package ports

import (
	"context"
	"time"

	sdkEntities "github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	PluginName     string
	IsProfileBased bool
	IsBroad        bool
}

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime PluginRuntime, pluginDir string) (map[string]*sdkEntities.GrantSet, error)
}

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This allows the orchestrator to be tested with mock analyzers.
type CapabilityAnalyzer interface {
	ExtractCapabilities(profile entities.ProfileReader) map[string]*sdkEntities.GrantSet
}

// CapabilityGatekeeperPort grants capabilities based on security policy.
// Named with "Port" suffix to avoid collision with the concrete CapabilityGatekeeper type.
type CapabilityGatekeeperPort interface {
	GrantCapabilities(
		required *sdkEntities.GrantSet,
		capabilityInfo map[string]CapabilityInfo,
		trustAll bool,
	) (*sdkEntities.GrantSet, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(ctx context.Context, required map[string]*sdkEntities.GrantSet, trustAll bool) (map[string]*sdkEntities.GrantSet, error)
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
		requiredCaps map[string]*sdkEntities.GrantSet,
		trustFlag bool,
	) (bool, error)
}

// GrantStore persists and retrieves granted capabilities.
type GrantStore interface {
	Load() (*sdkEntities.GrantSet, error)
	Save(grants *sdkEntities.GrantSet) error
	ConfigPath() string
}

// Prompter handles interactive capability authorization.
type Prompter interface {
	IsInteractive() bool
	PromptForCapability(req sdkEntities.CapabilityRequest) (granted bool, always bool, err error)
	PromptForCapabilities(reqs []sdkEntities.CapabilityRequest) (*sdkEntities.GrantSet, error)
	FormatNonInteractiveError(missing *sdkEntities.GrantSet) error
}
