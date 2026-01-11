package ports

import "context"

// AuthProvider retrieves authentication credentials for registries.
type AuthProvider interface {
	// GetCredentials returns username and password for a registry.
	GetCredentials(ctx context.Context, registry string) (username, password string, err error)
}
