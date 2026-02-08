package ports

import (
	"context"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	hostSDK "github.com/reglet-dev/reglet-host-sdk/capability"
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo = hostSDK.CapabilityInfo

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime PluginRuntime, pluginDir string) (map[string]*hostfunc.GrantSet, error)
}

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This allows the orchestrator to be tested with mock analyzers.
type CapabilityAnalyzer interface {
	ExtractCapabilities(profile entities.ProfileReader) map[string]*hostfunc.GrantSet
}

// CapabilityGatekeeperPort grants capabilities based on security policy.
// Named with "Port" suffix to avoid collision with the concrete CapabilityGatekeeper type.
type CapabilityGatekeeperPort interface {
	GrantCapabilities(
		required *hostfunc.GrantSet,
		capabilityInfo map[string]CapabilityInfo,
		trustAll bool,
	) (*hostfunc.GrantSet, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(ctx context.Context, required map[string]*hostfunc.GrantSet, trustAll bool) (map[string]*hostfunc.GrantSet, error)
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
		requiredCaps map[string]*hostfunc.GrantSet,
		trustFlag bool,
	) (bool, error)
}

// GrantStore persists and retrieves granted capabilities.
type GrantStore interface {
	Load() (*hostfunc.GrantSet, error)
	Save(grants *hostfunc.GrantSet) error
	ConfigPath() string
}

// Prompter handles interactive capability authorization.
type Prompter interface {
	IsInteractive() bool
	PromptForCapability(req hostSDK.Request) (granted bool, always bool, err error)
	PromptForCapabilities(reqs []hostSDK.Request) (*hostfunc.GrantSet, error)
	FormatNonInteractiveError(missing *hostfunc.GrantSet) error
}
