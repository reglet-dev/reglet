package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func TestFilePlugin_Check_Exists(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	plugin := &filePlugin{}
	config := regletsdk.Config{
		"path": tmpFile,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false. Error: %v", evidence.Error)
	}

	// Verify metadata
	if exists, ok := evidence.Data["exists"].(bool); !ok || !exists {
		t.Errorf("Expected exists=true, got %v", evidence.Data["exists"])
	}
	if isDir, ok := evidence.Data["is_dir"].(bool); !ok || isDir {
		t.Errorf("Expected is_dir=false, got %v", evidence.Data["is_dir"])
	}
	if size, ok := evidence.Data["size"].(int64); !ok || size != 7 {
		t.Errorf("Expected size=7, got %v", evidence.Data["size"])
	}
}

func TestFilePlugin_Check_Content(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	content := "hello world"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	plugin := &filePlugin{}
	config := regletsdk.Config{
		"path":         tmpFile,
		"read_content": true,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}

	b64, ok := evidence.Data["content_b64"].(string)
	if !ok {
		t.Errorf("Expected content_b64 string")
	}
	if b64 == "" {
		t.Errorf("Expected non-empty content")
	}
}

func TestFilePlugin_Check_Hash(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	content := "test hash"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	plugin := &filePlugin{}
	config := regletsdk.Config{
		"path": tmpFile,
		"hash": true,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}

	// echo -n "test hash" | sha256sum
	// 54a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b
	expectedHash := "54a6483b8aca55c9df2a35baf71d9965ddfd623468d81d51229bd5eb7d1e1c1b"
	
	hash, ok := evidence.Data["sha256"].(string)
	if !ok {
		t.Errorf("Expected sha256 string")
	}
	if hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, hash)
	}
}

func TestFilePlugin_Check_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "missing")

	plugin := &filePlugin{}
	config := regletsdk.Config{
		"path": tmpFile,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	// Status should be TRUE (success), but exists=false
	if !evidence.Status {
		t.Errorf("Expected status true for non-existent file check, got false. Error: %v", evidence.Error)
	}

	if exists, ok := evidence.Data["exists"].(bool); !ok || exists {
		t.Errorf("Expected exists=false, got %v", evidence.Data["exists"])
	}
}

func TestFilePlugin_Check_MissingPath(t *testing.T) {
	plugin := &filePlugin{}
	config := regletsdk.Config{}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	
	if evidence.Status {
		t.Error("Expected status false for missing path config")
	}
	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}
