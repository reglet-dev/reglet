package services

import (
	"context"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

func TestIntegrityService(t *testing.T) {
	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")
	meta := values.NewPluginMetadata("name", "1.0", "desc", nil)
	digest, _ := values.NewDigest("sha256", "abc")

	// Plugin with digest "abc"
	plugin := entities.NewPlugin(ref, digest, meta)

	t.Run("VerifyDigest_Success", func(t *testing.T) {
		svc := NewIntegrityService(false)
		// Expected matches plugin
		err := svc.VerifyDigest(plugin, digest)
		if err != nil {
			t.Errorf("VerifyDigest failed: %v", err)
		}
	})

	t.Run("VerifyDigest_Mismatch", func(t *testing.T) {
		svc := NewIntegrityService(false)
		otherDigest, _ := values.NewDigest("sha256", "def")

		err := svc.VerifyDigest(plugin, otherDigest)
		if err == nil {
			t.Error("VerifyDigest should fail on mismatch")
		}
	})

	t.Run("ShouldVerifySignature", func(t *testing.T) {
		svcTrue := NewIntegrityService(true)
		if !svcTrue.ShouldVerifySignature() {
			t.Error("Should return true")
		}

		svcFalse := NewIntegrityService(false)
		if svcFalse.ShouldVerifySignature() {
			t.Error("Should return false")
		}
	})

	t.Run("ValidatePlugin_DigestCheck", func(t *testing.T) {
		svc := NewIntegrityService(false)

		err := svc.ValidatePlugin(context.Background(), plugin, digest)
		if err != nil {
			t.Errorf("ValidatePlugin failed: %v", err)
		}

		badDigest, _ := values.NewDigest("sha256", "bad")
		err = svc.ValidatePlugin(context.Background(), plugin, badDigest)
		if err == nil {
			t.Error("ValidatePlugin should fail on bad digest")
		}
	})
}
