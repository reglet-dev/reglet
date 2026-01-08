# Evidence Redaction

Reglet includes a built-in redaction system to sanitize sensitive data (secrets, PII, credentials) from plugin output before it is stored or displayed. This ensures audit artifacts are safe to share with auditors or store in logs.

## How It Works

Redaction uses a **two-phase approach**:

1. **Gitleaks Detection** — Industry-standard secret detection covering AWS keys, GitHub tokens, JWTs, Stripe keys, private keys, and more
2. **Custom Pattern Matching** — Your own regex patterns for organization-specific secrets

Both phases are applied automatically to all plugin output.

## Configuration

Redaction is configured in your system configuration file (usually `~/.reglet/config.yaml`).

```yaml
redaction:
  # Replace sensitive values with a consistent hash instead of [REDACTED]
  hash_mode:
    enabled: false
    # Optional: salt for hashing (prevents rainbow table attacks)
    salt: "my-secret-salt-string"

  # List of regex patterns to scrub from any string output
  patterns:
    - "SECRET-[A-Z0-9]{8}"
    - "API_KEY=[a-zA-Z0-9]+"

  # List of JSON paths (keys) to always redact
  paths:
    - "password"
    - "secret_key"
    - "token"
    - "auth.bearer"

  # Disable gitleaks detector (use only custom patterns)
  # Default: false (gitleaks enabled)
  disable_gitleaks: false
```

### 1. Gitleaks Detection (Default)

By default, Reglet uses the [gitleaks](https://github.com/gitleaks/gitleaks) library to detect secret patterns including:

- AWS Access Keys, Secret Keys, Session Tokens
- GitHub/GitLab/Bitbucket tokens
- Private keys (RSA, DSA, EC, PGP)
- JWTs and Bearer tokens
- Stripe, Slack, Twilio, SendGrid API keys
- Database connection strings
- And many more...

This provides comprehensive coverage without configuration.

**To disable gitleaks** (use only custom patterns):
```yaml
redaction:
  disable_gitleaks: true
```

### 2. Custom Pattern Matching (Regex)

You can define regular expressions to find and redact organization-specific secrets.

**Example:**
If you define `patterns: ["MySecret-[0-9]+"]`, the text:
`"The code is MySecret-12345"`
becomes:
`"The code is [REDACTED]"`

### 3. Path Matching (Structural)

For structured data (JSON results from plugins), you can redact values based on their key name. This is faster and safer than regex for known fields.

**Example:**
If you define `paths: ["password"]`, any field named `password` in the evidence map will be redacted, regardless of its value.

```json
// Before
{
  "username": "admin",
  "password": "super-secret-password-123"
}

// After
{
  "username": "admin",
  "password": "[REDACTED]"
}
```

### 4. Hash Mode

By default, secrets are replaced with the static string `[REDACTED]`. 

If you enable `hash_mode`, secrets are replaced with a truncated HMAC-SHA256 hash of the value. This allows you to correlate secrets (prove they are the same across two different checks) without revealing the secret itself.

**Config:**
```yaml
redaction:
  hash_mode:
    enabled: true
    salt: "random-string-generated-by-you"
```

**Output:**
```
[hmac:a1b2c3d4e5f6g7h8]
```

**Salting:**
To prevent rainbow table attacks, always configure a **salt**. This ensures that the hash of "password123" is unique to your organization.

## Best Practices

1. **Leave Gitleaks Enabled**: The patterns cover most common secrets with minimal false positives
2. **Add Custom Patterns**: Use `patterns` for organization-specific secrets (internal IDs, custom tokens)
3. **Use Path Matching for Known Fields**: Faster and more precise than regex for structured data
4. **Use Hash Mode for Audits**: Useful when an auditor asks "Is the API key on Server A the same as Server B?"
5. **Always Use a Salt**: If using Hash Mode, configure a secret salt to prevent reversing the hashes