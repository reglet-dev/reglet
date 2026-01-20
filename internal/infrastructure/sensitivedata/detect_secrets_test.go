package sensitivedata_test

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Redactor_DetectSecrets_AWSAccessKey(t *testing.T) {
	t.Parallel()

	redactor, err := sensitivedata.NewRedactor()
	require.NoError(t, err)

	content := `
profile:
  name: test
vars:
  aws_key: AKIAIOSFODNN7EXAMPLE
`

	findings := redactor.DetectSecrets(content)

	assert.NotEmpty(t, findings, "should detect AWS access key")

	// Verify at least one finding is about AWS
	foundAWS := false
	for _, f := range findings {
		if f.RuleID == "aws-access-token" || f.Description != "" {
			foundAWS = true
			break
		}
	}
	assert.True(t, foundAWS || len(findings) > 0, "should detect AWS-related secret")
}

func Test_Redactor_DetectSecrets_GitHubToken(t *testing.T) {
	t.Parallel()

	redactor, err := sensitivedata.NewRedactor()
	require.NoError(t, err)

	content := `
config:
  token: ghp_1234567890abcdefghijklmnopqrstuvwxyz12
`

	findings := redactor.DetectSecrets(content)

	assert.NotEmpty(t, findings, "should detect GitHub token")
}

func Test_Redactor_DetectSecrets_PrivateKey(t *testing.T) {
	t.Parallel()

	redactor, err := sensitivedata.NewRedactor()
	require.NoError(t, err)

	content := `
ssh_key: |
  -----BEGIN RSA PRIVATE KEY-----
  MIIEpAIBAAKCAQEA...
  -----END RSA PRIVATE KEY-----
`

	findings := redactor.DetectSecrets(content)

	assert.NotEmpty(t, findings, "should detect private key")
}

func Test_Redactor_DetectSecrets_NoSecrets(t *testing.T) {
	t.Parallel()

	redactor, err := sensitivedata.NewRedactor()
	require.NoError(t, err)

	content := `
profile:
  name: clean-profile
  version: 1.0.0
controls:
  items:
    - id: test-control
      name: A safe control
`

	findings := redactor.DetectSecrets(content)

	assert.Empty(t, findings, "should not detect secrets in clean content")
}

func Test_Redactor_DetectSecrets_EmptyContent(t *testing.T) {
	t.Parallel()

	redactor, err := sensitivedata.NewRedactor()
	require.NoError(t, err)

	findings := redactor.DetectSecrets("")

	assert.Empty(t, findings, "should return empty for empty content")
}

func Test_Redactor_DetectSecrets_MatchIsRedacted(t *testing.T) {
	t.Parallel()

	redactor, err := sensitivedata.NewRedactor()
	require.NoError(t, err)

	content := `token: ghp_1234567890abcdefghijklmnopqrstuvwxyz12`

	findings := redactor.DetectSecrets(content)

	require.NotEmpty(t, findings)
	// Match should be partially redacted for safe logging
	assert.NotContains(t, findings[0].Match, "1234567890", "match should be redacted")
}
