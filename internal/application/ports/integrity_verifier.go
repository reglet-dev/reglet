package ports

import (
	"context"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

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
