package redaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedactor_GitleaksIntegration verifies that the gitleaks library
// is properly integrated and provides comprehensive secret detection.
func TestRedactor_GitleaksIntegration(t *testing.T) {
	// Create redactor with gitleaks enabled (default)
	redactor, err := New(Config{})
	require.NoError(t, err)
	require.NotNil(t, redactor.gitleaksDetector, "Gitleaks detector should be initialized by default")

	tests := []struct {
		name     string
		input    string
		expected string
		shouldRedact bool
	}{
		{
			name:         "GitHub Personal Access Token",
			input:        "export GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuv",
			shouldRedact: true,
		},
		{
			name:         "Stripe API Key",
			input:        "STRIPE_KEY=sk_test_4eC39HqLyjWDarjtT1zdp7dc",
			shouldRedact: true,
		},
		{
			name:         "JWT Token",
			input:        "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			shouldRedact: true,
		},
		{
			name:         "Slack Token",
			input:        "SLACK_TOKEN=xoxb-123456789012-1234567890123-1234567890123456789012",
			shouldRedact: true,
		},
		{
			name:         "Generic API Key",
			input:        "api_key=sk_test_1234567890abcdef",
			shouldRedact: true,
		},
		{
			name:         "Normal Text",
			input:        "This is just normal text without any secrets",
			shouldRedact: false,
		},
		{
			name:         "Normal Email",
			input:        "Contact: user@example.com",
			shouldRedact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.ScrubString(tt.input)

			if tt.shouldRedact {
				assert.NotEqual(t, tt.input, result, "Input should be modified")
				assert.Contains(t, result, "[REDACTED]", "Should contain redaction marker")
				t.Logf("Original: %s", tt.input)
				t.Logf("Redacted: %s", result)
			} else {
				assert.Equal(t, tt.input, result, "Normal text should not be modified")
			}
		})
	}
}

// TestRedactor_GitleaksDisabled verifies that redaction works when gitleaks is disabled.
func TestRedactor_GitleaksDisabled(t *testing.T) {
	// Create redactor with gitleaks disabled
	redactor, err := New(Config{
		DisableGitleaks: true,
		Patterns: []string{
			`test-secret-[0-9a-f]{8}`, // Custom pattern
		},
	})
	require.NoError(t, err)
	require.Nil(t, redactor.gitleaksDetector, "Gitleaks detector should be nil when disabled")

	// Should still redact using custom patterns
	input := "My secret is test-secret-12345678"
	result := redactor.ScrubString(input)
	assert.Contains(t, result, "[REDACTED]", "Should redact using custom patterns")
	assert.NotEqual(t, input, result, "Should be modified")
}

// TestRedactor_CoverageComparison demonstrates the improved coverage with gitleaks.
func TestRedactor_CoverageComparison(t *testing.T) {
	// Redactor WITHOUT gitleaks (only 4 default patterns)
	redactorWithout, err := New(Config{
		DisableGitleaks: true,
	})
	require.NoError(t, err)

	// Redactor WITH gitleaks (222+ patterns)
	redactorWith, err := New(Config{
		DisableGitleaks: false,
	})
	require.NoError(t, err)

	testCases := []string{
		"STRIPE_KEY=sk_test_4eC39HqLyjWDarjtT1zdp7dc",
		"ANTHROPIC_API_KEY=sk-ant-api03-" + "X",
		"JWT=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		"SENDGRID_API_KEY=SG.1234567890abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNO",
	}

	redactedWithout := 0
	redactedWith := 0

	for _, input := range testCases {
		resultWithout := redactorWithout.ScrubString(input)
		resultWith := redactorWith.ScrubString(input)

		if resultWithout != input {
			redactedWithout++
		}
		if resultWith != input {
			redactedWith++
		}
	}

	t.Logf("Redacted WITHOUT gitleaks: %d/%d", redactedWithout, len(testCases))
	t.Logf("Redacted WITH gitleaks: %d/%d", redactedWith, len(testCases))

	// Gitleaks should catch more secrets
	assert.GreaterOrEqual(t, redactedWith, redactedWithout,
		"Gitleaks should catch at least as many secrets as default patterns")
}

// TestRedactor_HashModeWithGitleaks verifies hash mode works with gitleaks.
func TestRedactor_HashModeWithGitleaks(t *testing.T) {
	redactor, err := New(Config{
		HashMode: true,
		Salt:     "test-salt-12345",
	})
	require.NoError(t, err)

	input := "GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuv"
	result := redactor.ScrubString(input)

	assert.NotEqual(t, input, result, "Should be modified")
	assert.Contains(t, result, "[hmac:", "Should use hash mode")
	assert.NotContains(t, result, "[REDACTED]", "Should not use redacted marker in hash mode")
	t.Logf("Hash mode result: %s", result)
}
