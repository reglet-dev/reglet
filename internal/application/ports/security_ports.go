package ports

import (
	"context"

	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	PluginName     string
	IsProfileBased bool
	IsBroad        bool
}

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime PluginRuntime, pluginDir string) (map[string]capability.GrantSet, error)
}

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This allows the orchestrator to be tested with mock analyzers.
type CapabilityAnalyzer interface {
	ExtractCapabilities(profile entities.ProfileReader) map[string]capability.GrantSet
}

// CapabilityGatekeeperPort grants capabilities based on security policy.
// Named with "Port" suffix to avoid collision with the concrete CapabilityGatekeeper type.
type CapabilityGatekeeperPort interface {
	GrantCapabilities(
		required capability.GrantSet,
		capabilityInfo map[string]CapabilityInfo,
		trustAll bool,
	) (capability.GrantSet, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(ctx context.Context, required map[string]capability.GrantSet, trustAll bool) (map[string]capability.GrantSet, error)
}

// DataRedactor scrubs sensitive information from output.
// Placed here as it's often used with execution/security components.
type DataRedactor interface {
	Redact(input string) string
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
		requiredCaps map[string]capability.GrantSet,
		trustFlag bool,
	) (bool, error)
}

// GrantStore persists and retrieves granted capabilities.
type GrantStore interface {
	Load() (capability.GrantSet, error)
	Save(grants capability.GrantSet) error
	ConfigPath() string
}

// Prompter handles interactive capability authorization.
type Prompter interface {
	IsInteractive() bool
	PromptForCapability(req capability.Request) (granted bool, always bool, err error)
	PromptForCapabilities(reqs []capability.Request) (capability.GrantSet, error)
	FormatNonInteractiveError(missing capability.GrantSet) error
}
