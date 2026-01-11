// Package oci implements OCI registry adapters.
package oci

import (
	"context"
	"os"
)

// EnvAuthProvider retrieves credentials from environment variables.
type EnvAuthProvider struct{}

// NewEnvAuthProvider creates a new environment-based auth provider.
func NewEnvAuthProvider() *EnvAuthProvider {
	return &EnvAuthProvider{}
}

// GetCredentials returns username and password for a registry.
func (p *EnvAuthProvider) GetCredentials(ctx context.Context, registry string) (username, password string, err error) {
	username = os.Getenv("REGISTRY_USERNAME")
	password = os.Getenv("REGISTRY_PASSWORD")
	return username, password, nil
}
