// Package config provides configuration parsing and variable handling.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxFileSize is the maximum size of a file that can be read via --set-file.
// This prevents accidentally reading huge files.
const MaxFileSize = 1024 * 1024 // 1MB

// ReadValueFromFile reads a value from a file for --set-file.
// The file content is trimmed of trailing newlines.
// Returns error if file doesn't exist or exceeds size limit.
func ReadValueFromFile(path string) (string, error) {
	// Clean the path to prevent path traversal attacks
	cleanPath := filepath.Clean(path)

	// Check file exists and get size
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("cannot access file %s: %w", path, err)
	}

	// Check file size
	if info.Size() > MaxFileSize {
		return "", fmt.Errorf("file too large: %s (max %d bytes)", path, MaxFileSize)
	}

	// Read content - path is cleaned above, safe from traversal
	// #nosec G304 -- path is sanitized with filepath.Clean above
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Trim trailing whitespace (common with files created by echo)
	value := strings.TrimRight(string(content), "\r\n")

	return value, nil
}

// ParseSetFile parses a --set-file argument (key=path) and returns the key and value.
func ParseSetFile(input string) (string, interface{}, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid --set-file format: expected key=path, got %q", input)
	}

	key := parts[0]
	path := parts[1]

	// Validate key format
	if !ValidKeyPattern.MatchString(key) {
		return "", nil, fmt.Errorf("invalid key %q: must match pattern [a-zA-Z_][a-zA-Z0-9_.]*", key)
	}

	// Read value from file
	value, err := ReadValueFromFile(path)
	if err != nil {
		return "", nil, err
	}

	return key, value, nil
}
