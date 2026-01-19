package profiles

import (
	"context"
	"encoding/base64"
	"strings"
)

// HeaderAuthProvider implements ProfileAuthProvider with configurable auth headers.
// Supports Bearer tokens, Basic auth, and custom Authorization headers.
type HeaderAuthProvider struct {
	// Rules maps URL patterns to auth configurations.
	// Patterns are matched from most specific to least specific.
	Rules []AuthRule
}

// AuthRule defines authentication for URLs matching a pattern.
type AuthRule struct {
	// Pattern is a URL prefix to match (e.g., "https://example.com/").
	// More specific patterns take precedence.
	Pattern string

	// AuthType is the type of authentication: "bearer", "basic", or "header".
	AuthType string

	// Token is used for bearer auth.
	Token string

	// Username and Password are used for basic auth.
	Username string
	Password string

	// HeaderValue is the raw Authorization header value for "header" type.
	HeaderValue string
}

// NewHeaderAuthProvider creates a new provider with the given rules.
func NewHeaderAuthProvider(rules []AuthRule) *HeaderAuthProvider {
	return &HeaderAuthProvider{Rules: rules}
}

// GetAuthHeader returns the Authorization header value for the given URL.
// Returns empty string if no auth is configured for this URL.
func (p *HeaderAuthProvider) GetAuthHeader(ctx context.Context, url string) (string, error) {
	// Find the most specific matching rule
	var bestMatch *AuthRule
	var bestMatchLen int

	for i := range p.Rules {
		rule := &p.Rules[i]
		if strings.HasPrefix(url, rule.Pattern) {
			if len(rule.Pattern) > bestMatchLen {
				bestMatch = rule
				bestMatchLen = len(rule.Pattern)
			}
		}
	}

	if bestMatch == nil {
		return "", nil // No auth configured
	}

	return p.buildAuthHeader(bestMatch), nil
}

// buildAuthHeader constructs the Authorization header based on auth type.
func (p *HeaderAuthProvider) buildAuthHeader(rule *AuthRule) string {
	switch strings.ToLower(rule.AuthType) {
	case "bearer":
		return "Bearer " + rule.Token

	case "basic":
		credentials := rule.Username + ":" + rule.Password
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
		return "Basic " + encoded

	case "header":
		return rule.HeaderValue

	default:
		return ""
	}
}

// StaticHeaderAuthProvider is a simple implementation that returns a fixed header
// for all URLs. Useful for simple use cases with a single auth token.
type StaticHeaderAuthProvider struct {
	Header string
}

// NewStaticBearerAuthProvider creates a provider with a fixed Bearer token.
func NewStaticBearerAuthProvider(token string) *StaticHeaderAuthProvider {
	return &StaticHeaderAuthProvider{
		Header: "Bearer " + token,
	}
}

// NewStaticBasicAuthProvider creates a provider with fixed Basic auth credentials.
func NewStaticBasicAuthProvider(username, password string) *StaticHeaderAuthProvider {
	credentials := username + ":" + password
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	return &StaticHeaderAuthProvider{
		Header: "Basic " + encoded,
	}
}

// GetAuthHeader returns the static header for any URL.
func (p *StaticHeaderAuthProvider) GetAuthHeader(ctx context.Context, url string) (string, error) {
	return p.Header, nil
}

// NoAuthProvider is an auth provider that provides no authentication.
// Useful as a default or for public profiles.
type NoAuthProvider struct{}

// GetAuthHeader always returns empty string.
func (p *NoAuthProvider) GetAuthHeader(ctx context.Context, url string) (string, error) {
	return "", nil
}

// ChainAuthProvider tries multiple providers in order until one returns a non-empty header.
type ChainAuthProvider struct {
	Providers []interface{ GetAuthHeader(context.Context, string) (string, error) }
}

// NewChainAuthProvider creates a provider that chains multiple providers.
func NewChainAuthProvider(providers ...interface{ GetAuthHeader(context.Context, string) (string, error) }) *ChainAuthProvider {
	return &ChainAuthProvider{Providers: providers}
}

// GetAuthHeader tries each provider in order until one returns a non-empty header.
func (p *ChainAuthProvider) GetAuthHeader(ctx context.Context, url string) (string, error) {
	for _, provider := range p.Providers {
		header, err := provider.GetAuthHeader(ctx, url)
		if err != nil {
			return "", err
		}
		if header != "" {
			return header, nil
		}
	}
	return "", nil
}
