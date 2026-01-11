package values

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Digest represents a content hash with algorithm.
type Digest struct {
	algorithm string // sha256, sha512
	value     string // hex-encoded hash
}

// NewDigest creates a digest from algorithm and hex value.
func NewDigest(algorithm, hexValue string) (Digest, error) {
	switch algorithm {
	case "sha256", "sha512":
		// Valid
	default:
		return Digest{}, fmt.Errorf("unsupported digest algorithm: %s", algorithm)
	}

	return Digest{
		algorithm: algorithm,
		value:     hexValue,
	}, nil
}

// ParseDigest parses a digest string (e.g., "sha256:abc123...").
func ParseDigest(s string) (Digest, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Digest{}, fmt.Errorf("invalid digest format: %s", s)
	}
	return NewDigest(parts[0], parts[1])
}

// String returns the canonical digest string.
func (d Digest) String() string {
	return fmt.Sprintf("%s:%s", d.algorithm, d.value)
}

// Algorithm returns the hash algorithm.
func (d Digest) Algorithm() string {
	return d.algorithm
}

// Value returns the hex-encoded hash value.
func (d Digest) Value() string {
	return d.value
}

// Equals checks equality with another digest.
func (d Digest) Equals(other Digest) bool {
	return d.algorithm == other.algorithm && d.value == other.value
}

// Verify validates data matches this digest.
func (d Digest) Verify(data []byte) error {
	computed, err := d.computeHash(data)
	if err != nil {
		return err
	}

	if !d.Equals(computed) {
		return fmt.Errorf("digest mismatch: expected %s, got %s", d.String(), computed.String())
	}

	return nil
}

// computeHash computes hash of data using this digest's algorithm.
func (d Digest) computeHash(data []byte) (Digest, error) {
	switch d.algorithm {
	case "sha256":
		hash := sha256.Sum256(data)
		return Digest{algorithm: "sha256", value: hex.EncodeToString(hash[:])}, nil
	case "sha512":
		hash := sha512.Sum512(data)
		return Digest{algorithm: "sha512", value: hex.EncodeToString(hash[:])}, nil
	default:
		return Digest{}, fmt.Errorf("unsupported algorithm: %s", d.algorithm)
	}
}

// ComputeDigestSHA256 computes SHA-256 digest of reader contents.
func ComputeDigestSHA256(r io.Reader) (Digest, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return Digest{}, err
	}
	return Digest{
		algorithm: "sha256",
		value:     hex.EncodeToString(h.Sum(nil)),
	}, nil
}
