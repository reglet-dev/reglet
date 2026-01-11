package services

import (
	"context"
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// IntegrityService provides domain logic for plugin integrity verification.
type IntegrityService struct {
	requireSigning bool
}

// NewIntegrityService creates an integrity service.
func NewIntegrityService(requireSigning bool) *IntegrityService {
	return &IntegrityService{
		requireSigning: requireSigning,
	}
}

// VerifyDigest checks if plugin digest matches expected value.
func (s *IntegrityService) VerifyDigest(plugin *entities.Plugin, expected values.Digest) error {
	return plugin.VerifyIntegrity(expected)
}

// ShouldVerifySignature returns true if signature verification is required.
func (s *IntegrityService) ShouldVerifySignature() bool {
	return s.requireSigning
}

// ValidatePlugin performs complete integrity check.
func (s *IntegrityService) ValidatePlugin(
	ctx context.Context,
	plugin *entities.Plugin,
	expectedDigest values.Digest,
) error {
	// Digest verification (always required)
	if err := s.VerifyDigest(plugin, expectedDigest); err != nil {
		return fmt.Errorf("digest verification failed: %w", err)
	}

	// Signature verification is delegated to application layer port
	// (domain doesn't know about cryptographic operations)

	return nil
}
