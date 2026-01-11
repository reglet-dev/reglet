package services

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

func TestPluginService_LoadPlugin(t *testing.T) {
	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")
	meta := values.NewPluginMetadata("name", "1.0", "desc", nil)
	digest, _ := values.NewDigest("sha256", "abc")
	plugin := entities.NewPlugin(ref, digest, meta)

	// Mock strategy that returns a plugin
	// Rename variable to avoid shadowing type mockResolver
	resolver := &mockResolver{foundPlugin: plugin}

	t.Run("Success_NoVerification", func(t *testing.T) {
		repo := &MockRepository{FindPath: "/path/to/wasm"}
		svc := NewPluginService(
			resolver,
			repo,
			nil,
			nil,
			services.NewIntegrityService(false),
			nil,
		)

		spec := &dto.PluginSpecDTO{Name: "reg/org/repo/name:1.0"}
		path, err := svc.LoadPlugin(context.Background(), spec)
		if err != nil {
			t.Fatalf("LoadPlugin failed: %v", err)
		}
		if path != "/path/to/wasm" {
			t.Errorf("expected path /path/to/wasm, got %s", path)
		}
	})

	t.Run("Success_WithDigestVerification", func(t *testing.T) {
		repo := &MockRepository{FindPath: "/path/to/wasm"}
		svc := NewPluginService(
			resolver,
			repo,
			nil,
			nil,
			services.NewIntegrityService(false),
			nil,
		)

		spec := &dto.PluginSpecDTO{Name: "reg/org/repo/name:1.0", Digest: "sha256:abc"}
		_, err := svc.LoadPlugin(context.Background(), spec)
		if err != nil {
			t.Errorf("LoadPlugin failed: %v", err)
		}
	})

	t.Run("Fail_DigestMismatch", func(t *testing.T) {
		repo := &MockRepository{FindPath: "/path/to/wasm"}
		svc := NewPluginService(
			resolver,
			repo,
			nil,
			nil,
			services.NewIntegrityService(false),
			nil,
		)

		spec := &dto.PluginSpecDTO{Name: "reg/org/repo/name:1.0", Digest: "sha256:bad"}
		_, err := svc.LoadPlugin(context.Background(), spec)
		if err == nil {
			t.Error("LoadPlugin should fail on digest mismatch")
		}
	})

	t.Run("Success_WithSignatureVerification", func(t *testing.T) {
		repo := &MockRepository{FindPath: "/path/to/wasm"}
		verifier := &MockVerifier{}
		svc := NewPluginService(
			resolver,
			repo,
			nil,
			verifier,
			services.NewIntegrityService(true), // Require signing
			NewTestLogger(),
		)

		spec := &dto.PluginSpecDTO{Name: "reg/org/repo/name:1.0"}
		_, err := svc.LoadPlugin(context.Background(), spec)
		if err != nil {
			t.Errorf("LoadPlugin failed: %v", err)
		}
	})

	t.Run("Fail_SignatureVerification", func(t *testing.T) {
		repo := &MockRepository{FindPath: "/path/to/wasm"}
		verifier := &MockVerifier{VerifyErr: errors.New("sig fail")}
		svc := NewPluginService(
			resolver,
			repo,
			nil,
			verifier,
			services.NewIntegrityService(true),
			NewTestLogger(),
		)

		spec := &dto.PluginSpecDTO{Name: "reg/org/repo/name:1.0"}
		_, err := svc.LoadPlugin(context.Background(), spec)
		if err == nil {
			t.Error("LoadPlugin should fail on signature error")
		}
	})

	t.Run("Fail_Resolution", func(t *testing.T) {
		// Here we want to use the type mockResolver to create a NEW instance
		badResolver := &mockResolver{err: errors.New("not found")}
		svc := NewPluginService(
			badResolver,
			&MockRepository{},
			nil,
			nil,
			services.NewIntegrityService(false),
			nil,
		)
		spec := &dto.PluginSpecDTO{Name: "reg/org/repo/name:1.0"}
		_, err := svc.LoadPlugin(context.Background(), spec)
		if err == nil {
			t.Error("LoadPlugin should fail on resolution error")
		}
	})
}

func TestPluginService_PublishPlugin(t *testing.T) {
	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")
	meta := values.NewPluginMetadata("name", "1.0", "desc", nil)
	digest, _ := values.NewDigest("sha256", "abc")
	plugin := entities.NewPlugin(ref, digest, meta)

	t.Run("Success_PushOnly", func(t *testing.T) {
		registry := &MockRegistry{}
		svc := NewPluginService(nil, nil, registry, nil, nil, NewTestLogger())

		err := svc.PublishPlugin(context.Background(), plugin, io.LimitReader(&mockReader{}, 0), false)
		if err != nil {
			t.Errorf("PublishPlugin failed: %v", err)
		}
	})

	t.Run("Success_PushAndSign", func(t *testing.T) {
		registry := &MockRegistry{}
		verifier := &MockVerifier{}
		svc := NewPluginService(nil, nil, registry, verifier, nil, NewTestLogger())

		err := svc.PublishPlugin(context.Background(), plugin, io.LimitReader(&mockReader{}, 0), true)
		if err != nil {
			t.Errorf("PublishPlugin failed: %v", err)
		}
	})
}

type mockReader struct{}

func (m *mockReader) Read(p []byte) (n int, err error) { return 0, io.EOF }
