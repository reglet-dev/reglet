package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExamples(t *testing.T) {
	t.Skip("Skipping examples due to OS dependency")
}

func TestFileService_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0o644))

	svc := &FileService{}

	// Case 1: File exists
	in1 := &CheckInput{Path: tmpFile, Operation: "exists"}
	out1, err := svc.CheckHandler(context.Background(), in1)
	require.NoError(t, err)
	assert.True(t, out1.Exists)

	// Case 2: File missing
	in2 := &CheckInput{Path: filepath.Join(tmpDir, "missing"), Operation: "exists"}
	out2, err := svc.CheckHandler(context.Background(), in2)
	require.NoError(t, err)
	assert.False(t, out2.Exists)
}

func TestFileService_Permissions(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	// 0644
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0o644))

	svc := &FileService{}

	// Case 1: Match
	in1 := &CheckInput{Path: tmpFile, Operation: "permissions", Permissions: "0644"}
	out1, err := svc.CheckHandler(context.Background(), in1)
	require.NoError(t, err)
	assert.Equal(t, "0644", out1.Permissions)

	in2 := &CheckInput{Path: tmpFile, Operation: "permissions", Permissions: "0600"}
	out2, err := svc.CheckHandler(context.Background(), in2)
	require.NoError(t, err)
	assert.Equal(t, "0644", out2.Permissions)
}

func TestFileService_Content(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("hello world"), 0o644))

	svc := &FileService{}

	// Case 1: Contains match
	in1 := &CheckInput{Path: tmpFile, Operation: "content", Contains: "world"}
	out1, err := svc.CheckHandler(context.Background(), in1)
	require.NoError(t, err)
	assert.True(t, out1.Contains)

	// Case 2: Contains mismatch
	in2 := &CheckInput{Path: tmpFile, Operation: "content", Contains: "mars"}
	out2, err := svc.CheckHandler(context.Background(), in2)
	require.NoError(t, err)
	assert.False(t, out2.Contains)
}

func TestFileService_Checksum(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	require.NoError(t, os.WriteFile(tmpFile, []byte("test hash"), 0o644))
	// echo -n "test hash" | sha256sum
	expectedHash := "54a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b"

	svc := &FileService{}

	// Case 1: Checksum
	in1 := &CheckInput{Path: tmpFile, Operation: "checksum"}
	out1, err := svc.CheckHandler(context.Background(), in1)
	require.NoError(t, err)
	assert.Equal(t, expectedHash, out1.Checksum)
}
