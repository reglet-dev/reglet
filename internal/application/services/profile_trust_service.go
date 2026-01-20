// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	infraCapabilities "github.com/reglet-dev/reglet/internal/infrastructure/capabilities"
)

// ProfileTrustService handles trust decisions for remote profiles.
// It determines whether a remote profile source is trusted and manages
// user prompts for untrusted sources.
type ProfileTrustService struct {
	prompter       *infraCapabilities.TerminalPrompter
	logger         *slog.Logger
	trustedSources []string // Glob patterns for trusted sources
}

// ProfileTrustServiceOption configures a ProfileTrustService.
type ProfileTrustServiceOption func(*ProfileTrustService)

// NewProfileTrustService creates a new profile trust service.
func NewProfileTrustService(opts ...ProfileTrustServiceOption) *ProfileTrustService {
	s := &ProfileTrustService{
		prompter:       infraCapabilities.NewTerminalPrompter(),
		trustedSources: nil,
		logger:         slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithTrustedSources sets the trusted source patterns.
func WithTrustedSources(patterns []string) ProfileTrustServiceOption {
	return func(s *ProfileTrustService) { s.trustedSources = patterns }
}

// WithTrustLogger sets a custom logger.
func WithTrustLogger(l *slog.Logger) ProfileTrustServiceOption {
	return func(s *ProfileTrustService) { s.logger = l }
}

// RequiresTrust returns true if the profile path is a remote URL
// that requires trust verification.
func (s *ProfileTrustService) RequiresTrust(path string) bool {
	return IsRemoteProfile(path)
}

// IsTrusted returns true if the URL matches a trusted source pattern.
func (s *ProfileTrustService) IsTrusted(url string) bool {
	if !s.RequiresTrust(url) {
		return true // Local profiles are always trusted
	}

	for _, pattern := range s.trustedSources {
		if s.matchPattern(pattern, url) {
			s.logger.Debug("profile matches trusted source", "url", url, "pattern", pattern)
			return true
		}
	}
	return false
}

// matchPattern checks if a URL matches a trusted source pattern.
// Supports glob-style patterns with * wildcards:
//   - "https://example.com/*" matches any path under example.com
//   - "https://*.example.com/*" matches any subdomain
//   - "https://example.com/profiles/*" matches only the profiles path
func (s *ProfileTrustService) matchPattern(pattern, url string) bool {
	// Handle exact match
	if pattern == url {
		return true
	}

	// Convert glob pattern to simple matching
	// Split pattern by * and check each segment
	segments := splitByWildcard(pattern)
	if len(segments) == 1 {
		// No wildcards - exact match only
		return pattern == url
	}

	// Check if URL matches the pattern segments
	remaining := url
	for i, seg := range segments {
		if seg == "" {
			continue // Skip empty segments from consecutive **
		}

		switch {
		case i == 0:
			// First segment must match at start
			if !hasPrefix(remaining, seg) {
				return false
			}
			remaining = remaining[len(seg):]
		case i == len(segments)-1:
			// Last segment must match at end (if not empty)
			if seg != "" && !hasSuffix(remaining, seg) {
				return false
			}
		default:
			// Middle segments must be found somewhere
			idx := indexOf(remaining, seg)
			if idx == -1 {
				return false
			}
			remaining = remaining[idx+len(seg):]
		}
	}

	return true
}

// splitByWildcard splits a pattern by * characters.
func splitByWildcard(pattern string) []string {
	var result []string
	current := ""
	for _, ch := range pattern {
		if ch == '*' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}

// hasPrefix checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// hasSuffix checks if s ends with suffix.
func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// indexOf returns the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// PromptForTrust prompts the user to trust a remote profile source.
// Shows the capabilities that the profile will require.
// Returns true if the user grants trust, false otherwise.
// Returns an error if running in non-interactive mode without --trust-source.
func (s *ProfileTrustService) PromptForTrust(
	ctx context.Context,
	url string,
	requiredCaps map[string][]capabilities.Capability,
	trustFlag bool,
) (bool, error) {
	// If trust flag is set, auto-trust
	if trustFlag {
		s.logger.Warn("auto-trusting remote profile (--trust-source enabled)", "url", url)
		return true, nil
	}

	// If already trusted via config, allow
	if s.IsTrusted(url) {
		s.logger.Debug("profile from trusted source", "url", url)
		return true, nil
	}

	// Check for non-interactive mode
	if !s.prompter.IsInteractive() {
		return false, s.FormatNonInteractiveError(url, requiredCaps)
	}

	// Display the remote profile info and prompt for trust
	return s.prompter.PromptForProfileTrust(url, requiredCaps)
}

// FormatNonInteractiveError creates a helpful error message for non-interactive mode.
func (s *ProfileTrustService) FormatNonInteractiveError(
	url string,
	requiredCaps map[string][]capabilities.Capability,
) error {
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("Remote profile requires trust approval: %s\n\n", url))
	msg.WriteString("Running in non-interactive mode. Cannot prompt for trust.\n\n")

	if len(requiredCaps) > 0 {
		msg.WriteString("Required capabilities:\n")
		for plugin, caps := range requiredCaps {
			for _, cap := range caps {
				msg.WriteString(fmt.Sprintf("  - [%s] %s\n", plugin, cap.String()))
			}
		}
		msg.WriteString("\n")
	}

	msg.WriteString("To run this remote profile:\n")
	msg.WriteString("  1. Run interactively and approve when prompted\n")
	msg.WriteString("  2. Use --trust-source flag to trust this source\n")
	msg.WriteString("  3. Add to trusted_profile_sources in ~/.reglet/config.yaml\n")

	return fmt.Errorf("%s", msg.String())
}

// TrustResult represents the result of a trust check.
type TrustResult struct {
	Trusted    bool
	FromConfig bool
	FromFlag   bool
}

// Ensure ProfileTrustService implements the port
var _ ports.ProfileTrustChecker = (*ProfileTrustService)(nil)
