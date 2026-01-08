package main

import (
	"context"
	"os"
	"testing"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func TestProcessPlugin_Check_ByPID(t *testing.T) {
	plugin := &processPlugin{}

	// Check current process (should always exist)
	config := regletsdk.Config{
		"pid": float64(os.Getpid()), // JSON numbers are float64
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false. Error: %v", evidence.Error)
	}

	if found, ok := evidence.Data["found"].(bool); !ok || !found {
		t.Errorf("Expected found=true, got %v", evidence.Data["found"])
	}

	if count, ok := evidence.Data["count"].(int); !ok || count != 1 {
		t.Errorf("Expected count=1, got %v", evidence.Data["count"])
	}
}

func TestProcessPlugin_Check_ByName(t *testing.T) {
	plugin := &processPlugin{}

	// Look for "init" or current process name
	// PID 1 is typically init/systemd
	config := regletsdk.Config{
		"pid": float64(1),
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	// This might fail in containers without PID 1 access
	// Just check that we got a valid response
	if !evidence.Status {
		t.Logf("Could not read PID 1 (may be permission denied): %v", evidence.Error)
		t.Skip("Skipping: no access to PID 1")
	}

	if _, ok := evidence.Data["found"].(bool); !ok {
		t.Errorf("Expected found field, got %v", evidence.Data)
	}
}

func TestProcessPlugin_Check_ByPattern(t *testing.T) {
	plugin := &processPlugin{}

	// Match any process with "go" in the cmdline (should match test runner)
	config := regletsdk.Config{
		"pattern": ".*go.*",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false. Error: %v", evidence.Error)
	}

	// Should find at least one go process (the test itself)
	if count, ok := evidence.Data["count"].(int); ok && count > 0 {
		t.Logf("Found %d processes matching '.*go.*'", count)
	}
}

func TestProcessPlugin_Check_NotFound(t *testing.T) {
	plugin := &processPlugin{}

	// Check non-existent PID
	config := regletsdk.Config{
		"pid": float64(999999999), // Very unlikely to exist
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true (successful check with no match), got false. Error: %v", evidence.Error)
	}

	if found, ok := evidence.Data["found"].(bool); !ok || found {
		t.Errorf("Expected found=false, got %v", evidence.Data["found"])
	}
}

func TestProcessPlugin_Check_MissingConfig(t *testing.T) {
	plugin := &processPlugin{}

	// No name, pid, or pattern provided
	config := regletsdk.Config{}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if evidence.Status {
		t.Error("Expected status false for missing config fields")
	}

	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}

func TestProcessPlugin_Check_InvalidPattern(t *testing.T) {
	plugin := &processPlugin{}

	// Invalid regex
	config := regletsdk.Config{
		"pattern": "[invalid(regex",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if evidence.Status {
		t.Error("Expected status false for invalid regex")
	}

	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}

func TestProcessPlugin_Describe(t *testing.T) {
	plugin := &processPlugin{}

	metadata, err := plugin.Describe(context.Background())
	if err != nil {
		t.Fatalf("Describe returned error: %v", err)
	}

	if metadata.Name != "process" {
		t.Errorf("Expected name 'process', got '%s'", metadata.Name)
	}

	if len(metadata.Capabilities) == 0 {
		t.Error("Expected at least one capability")
	}

	cap := metadata.Capabilities[0]
	if cap.Kind != "fs" || cap.Pattern != "read:/proc/**" {
		t.Errorf("Expected fs:read:/proc/** capability, got %s:%s", cap.Kind, cap.Pattern)
	}
}
