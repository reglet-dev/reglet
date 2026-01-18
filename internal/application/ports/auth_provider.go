package ports

import "context"

// AuthProvider retrieves authentication credentials for registries.
type AuthProvider interface {
	// GetCredentials returns username and password for a registry.
	GetCredentials(ctx context.Context, registry string) (username, password string, err error)
}

// ProfileAuthProvider retrieves authentication headers for remote profile fetching.
// Supports header-based authentication (Bearer, Basic, custom headers).
type ProfileAuthProvider interface {
	// GetAuthHeader returns the Authorization header value for the given URL.
	// Returns empty string if no auth is configured for this URL.
	GetAuthHeader(ctx context.Context, url string) (string, error)
}
