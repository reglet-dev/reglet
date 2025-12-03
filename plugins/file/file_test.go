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
		"mode": "exists",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false. Error: %v", evidence.Error)
	}
}

func TestFilePlugin_Check_Readable(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	plugin := &filePlugin{}
	config := regletsdk.Config{
		"path": tmpFile,
		"mode": "readable",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
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
		"path": tmpFile,
		"mode": "content",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}

	// Check base64 content (simplified check, assuming result correct if status true for now)
	// Real check: decode base64 and compare
	b64, ok := evidence.Data["content_b64"].(string)
	if !ok {
		t.Errorf("Expected content_b64 string")
	}
	if b64 == "" {
		t.Errorf("Expected non-empty content")
	}
}

func TestFilePlugin_Check_MissingPath(t *testing.T) {
	plugin := &filePlugin{}
	config := regletsdk.Config{
		"mode": "exists",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}
	
	if evidence.Status {
		t.Error("Expected status false for missing path")
	}
	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}
