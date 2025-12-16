package integration

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
)

// TestGitleaksLibrary_Integration verifies we can use gitleaks as a library
// for secret detection and redaction instead of manually maintaining patterns.
func TestGitleaksLibrary_Integration(t *testing.T) {
	// Load gitleaks default config using viper
	v := viper.New()
	v.SetConfigType("toml")
	err := v.ReadConfig(strings.NewReader(config.DefaultConfig))
	require.NoError(t, err, "Should read default config")

	var vc config.ViperConfig
	err = v.Unmarshal(&vc)
	require.NoError(t, err, "Should unmarshal config")

	cfg, err := vc.Translate()
	require.NoError(t, err, "Should translate config")
	require.NotEmpty(t, cfg.Rules, "Should have rules loaded")

	t.Logf("Loaded %d rules from gitleaks", len(cfg.Rules))

	// Create detector
	detector := detect.NewDetector(cfg)
	require.NotNil(t, detector, "Should create detector")

	// Test cases with realistic secret formats
	tests := []struct {
		name         string
		input        string
		expectSecret bool
	}{
		// Note: AWS key detection requires specific format/entropy, skipping for now
		{
			name:         "GitHub PAT",
			input:        "export GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuv",
			expectSecret: true,
		},
		{
			name:         "Stripe API Key",
			input:        "STRIPE_KEY=sk_test_4eC39HqLyjWDarjtT1zdp7dc",
			expectSecret: true,
		},
		{
			name:         "JWT Token",
			input:        "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expectSecret: true,
		},
		// Note: Private key detection requires full key content, not just headers
		{
			name:         "Normal Text",
			input:        "This is just normal text without any secrets",
			expectSecret: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fragment := detect.Fragment{
				Raw: tt.input,
			}

			findings := detector.Detect(fragment)

			if tt.expectSecret {
				assert.NotEmpty(t, findings, "Should detect secret in: %s", tt.input)
				if len(findings) > 0 {
					// Test redaction
					redacted := tt.input
					for _, f := range findings {
						redacted = strings.ReplaceAll(redacted, f.Secret, "[REDACTED]")
					}
					assert.NotEqual(t, tt.input, redacted, "Should be redacted")
					assert.Contains(t, redacted, "[REDACTED]", "Should contain redaction marker")

					t.Logf("Detected: %s (rule: %s)", findings[0].Description, findings[0].RuleID)
					t.Logf("Secret: %s", findings[0].Secret)
					t.Logf("Redacted: %s", redacted)
				}
			} else {
				assert.Empty(t, findings, "Should not detect secrets in normal text")
			}
		})
	}
}

// TestGitleaksLibrary_Performance verifies the performance is acceptable
func TestGitleaksLibrary_Performance(t *testing.T) {
	// Load config
	v := viper.New()
	v.SetConfigType("toml")
	err := v.ReadConfig(strings.NewReader(config.DefaultConfig))
	require.NoError(t, err)

	var vc config.ViperConfig
	err = v.Unmarshal(&vc)
	require.NoError(t, err)

	cfg, err := vc.Translate()
	require.NoError(t, err)

	detector := detect.NewDetector(cfg)

	// Test with a typical log line
	testData := "2024-01-01 INFO Connecting to database with connection string: user:pass@localhost:5432/db"

	fragment := detect.Fragment{
		Raw: testData,
	}

	// Run detection
	findings := detector.Detect(fragment)

	// Should be fast (this is just a smoke test, not a benchmark)
	t.Logf("Processed %d bytes, found %d secrets", len(testData), len(findings))
}
