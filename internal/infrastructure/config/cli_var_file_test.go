package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadValueFromFile(t *testing.T) {
	// Create temp dir for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		setup     func() string // Returns file path
		expected  string
		wantError bool
		errMsg    string
	}{
		{
			name: "simple content",
			setup: func() string {
				path := filepath.Join(tmpDir, "simple.txt")
				_ = os.WriteFile(path, []byte("secret-value"), 0o600)
				return path
			},
			expected: "secret-value",
		},
		{
			name: "content with trailing newline",
			setup: func() string {
				path := filepath.Join(tmpDir, "newline.txt")
				_ = os.WriteFile(path, []byte("secret-value\n"), 0o600)
				return path
			},
			expected: "secret-value",
		},
		{
			name: "content with multiple trailing newlines",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi-newline.txt")
				_ = os.WriteFile(path, []byte("secret-value\n\n\n"), 0o600)
				return path
			},
			expected: "secret-value",
		},
		{
			name: "content with Windows line ending",
			setup: func() string {
				path := filepath.Join(tmpDir, "windows.txt")
				_ = os.WriteFile(path, []byte("secret-value\r\n"), 0o600)
				return path
			},
			expected: "secret-value",
		},
		{
			name: "multiline content preserved except trailing",
			setup: func() string {
				path := filepath.Join(tmpDir, "multiline.txt")
				_ = os.WriteFile(path, []byte("line1\nline2\n"), 0o600)
				return path
			},
			expected: "line1\nline2",
		},
		{
			name: "empty file",
			setup: func() string {
				path := filepath.Join(tmpDir, "empty.txt")
				_ = os.WriteFile(path, []byte(""), 0o600)
				return path
			},
			expected: "",
		},
		{
			name: "file not found",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent.txt")
			},
			wantError: true,
			errMsg:    "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			got, err := ReadValueFromFile(path)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseSetFile(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	_ = os.WriteFile(secretFile, []byte("my-secret\n"), 0o600)

	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue interface{}
		wantError bool
		errMsg    string
	}{
		{
			name:      "valid key=path",
			input:     "api_key=" + secretFile,
			wantKey:   "api_key",
			wantValue: "my-secret",
		},
		{
			name:      "no equals sign",
			input:     "api_key",
			wantError: true,
			errMsg:    "invalid --set-file format",
		},
		{
			name:      "invalid key",
			input:     "123key=" + secretFile,
			wantError: true,
			errMsg:    "invalid key",
		},
		{
			name:      "file not found",
			input:     "key=/nonexistent/path/file.txt",
			wantError: true,
			errMsg:    "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, err := ParseSetFile(tt.input)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}
