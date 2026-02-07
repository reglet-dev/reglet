package ports

import "context"

// ProfileAuthProvider retrieves authentication headers for remote profile fetching.
// Supports header-based authentication (Bearer, Basic, custom headers).
type ProfileAuthProvider interface {
	// GetAuthHeader returns the Authorization header value for the given URL.
	// Returns empty string if no auth is configured for this URL.
	GetAuthHeader(ctx context.Context, url string) (string, error)
}
