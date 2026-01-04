package redaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactor_ScrubString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hashMode bool
		salt     string
		want     string
	}{
		{
			name:  "AWS Key Redaction",
			input: "My key is AKIAIOSFODNN7EXAMPLE",
			want:  "My key is [REDACTED]",
		},
		{
			name:  "Multiple Keys",
			input: "AKIAIOSFODNN7EXAMPLE and AKIAIOSFODNN7TESTING",
			want:  "[REDACTED] and [REDACTED]",
		},
		{
			name:  "No Secrets",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:     "Hash Mode (No Salt)",
			input:    "AKIAIOSFODNN7EXAMPLE",
			hashMode: true,
			want:     "[hmac:d3608e7190c42874c51ef490bdc7570d]", // HMAC-SHA256 of "AKIAIOSFODNN7EXAMPLE" with empty salt (first 16 bytes)
		},
		{
			name:     "Hash Mode (With Salt)",
			input:    "AKIAIOSFODNN7EXAMPLE",
			hashMode: true,
			salt:     "my-salt",
			// HMAC-SHA256 of "AKIAIOSFODNN7EXAMPLE" with key "my-salt"
			want: "[hmac:b9f2d1a41525d6f5899a386f50dc2295]", // First 16 bytes (32 hex chars)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(Config{
				HashMode: tt.hashMode,
				Salt:     tt.salt,
			})
			assert.NoError(t, err)
			got := r.ScrubString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedactor_Redact_Map(t *testing.T) {
	r, err := New(Config{
		Paths: []string{"password", "secret_key"},
	})
	assert.NoError(t, err)

	input := map[string]interface{}{
		"username": "admin",
		"password": "supersecret",
		"aws_key":  "AKIAIOSFODNN7EXAMPLE",
		"nested": map[string]interface{}{
			"secret_key": "hidden",
			"public":     "visible",
		},
	}

	expected := map[string]interface{}{
		"username": "admin",
		"password": "[REDACTED]", // Path match
		"aws_key":  "[REDACTED]", // Pattern match
		"nested": map[string]interface{}{
			"secret_key": "[REDACTED]", // Path match
			"public":     "visible",
		},
	}

	got := r.Redact(input)
	assert.Equal(t, expected, got)
}

func TestRedactor_Redact_Slice(t *testing.T) {
	r, err := New(Config{
		Paths: []string{"tokens"},
	})
	assert.NoError(t, err)

	// List of strings containing secrets
	input := []interface{}{
		"AKIAIOSFODNN7EXAMPLE",
		"safe string",
	}

	// Pattern match should work in slice
	expected := []interface{}{
		"[REDACTED]",
		"safe string",
	}

	got := r.Redact(input)
	assert.Equal(t, expected, got)
}
