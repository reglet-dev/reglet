package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/plugins/file/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileService_Exists_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0o644))

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path: tmpFile,
	}
	req := &plugin.Request{Config: cfg}

	result, err := svc.ExistsHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Contains(t, result.Message, "File exists")
}

func TestFileService_Exists_Fail(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "missing")

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path: tmpFile,
	}
	req := &plugin.Request{Config: cfg}

	// Based on implementation, ExistsHandler returns Failure if not exist
	result, err := svc.ExistsHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.False(t, result.Data["exists"].(bool))
}

func TestFileService_Permissions_Success(t *testing.T) {
	// On Windows checking permissions might be flaky, but this is Linux environment
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	// 0644
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0o644))

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path:        tmpFile,
		Permissions: "0644",
	}
	req := &plugin.Request{Config: cfg}

	result, err := svc.PermissionsHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}

func TestFileService_Permissions_Fail(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0o600))

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path:        tmpFile,
		Permissions: "0644",
	}
	req := &plugin.Request{Config: cfg}

	result, err := svc.PermissionsHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Contains(t, result.Message, "Permissions mismatch")
}

func TestFileService_Content_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("hello world"), 0o644))

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path:     tmpFile,
		Contains: "world",
	}
	req := &plugin.Request{Config: cfg}

	result, err := svc.ContentHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}

func TestFileService_Content_Fail(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("hello world"), 0o644))

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path:     tmpFile,
		Contains: "mars",
	}
	req := &plugin.Request{Config: cfg}

	result, err := svc.ContentHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusFailure, result.Status)
}

func TestFileService_Checksum_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test hash"), 0o644))
	// echo -n "test hash" | sha256sum
	expectedHash := "54a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b"

	svc := &FileService{}
	cfg := &core.FileConfig{
		Path:     tmpFile,
		Checksum: expectedHash,
	}
	req := &plugin.Request{Config: cfg}

	result, err := svc.ChecksumHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, expectedHash, result.Data["sha256"])
}
