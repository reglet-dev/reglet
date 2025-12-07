# Evidence Redaction

Reglet includes a built-in redaction system to sanitize sensitive data (secrets, PII, credentials) from plugin output before it is stored or displayed. This ensures audit artifacts are safe to share with auditors or store in logs.

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
```

### 1. Pattern Matching (Regex)

You can define regular expressions to find and redact secrets within text blocks (like log files or command output). Reglet comes with built-in patterns for common secrets (AWS keys, private keys, etc.), but you can add your own.

**Example:**
If you define `patterns: ["MySecret-[0-9]+"]`, the text:
`"The code is MySecret-12345"`
becomes:
`"The code is [REDACTED]"`

### 2. Path Matching (Structural)

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

### 3. Hash Mode

By default, secrets are replaced with the static string `[REDACTED]`. 

If you enable `hash_mode`, secrets are replaced with a truncated SHA-256 hash of the value. This allows you to correlate secrets (prove they are the same across two different checks) without revealing the secret itself.

**Config:**
```yaml
redaction:
  hash_mode:
    enabled: true
```

**Output:**
```
[sha256:a1b2c3d4]
```

**Salting:**
To prevent rainbow table attacks (where an attacker pre-computes hashes for common passwords), you should configure a **salt**. This ensures that the hash of "password123" is unique to your organization.

```yaml
redaction:
  hash_mode:
    enabled: true
    salt: "random-string-generated-by-you"
```

## Built-in Patterns

Reglet automatically attempts to detect and redact high-confidence secrets even without configuration, including:
- AWS Access Key IDs (`AKIA...`)
- Generic Private Key headers (`-----BEGIN RSA PRIVATE KEY-----`)
- GitHub Tokens (`ghp_...`)

## Best Practices

1.  **Prefer Path Matching**: Whenever possible, rely on path matching (`paths`) rather than regex. It is more performant and less prone to false positives.
2.  **Use Hash Mode for Audits**: Hash mode is useful when an auditor asks "Is the API key on Server A the same as Server B?". You can show them matching hashes without revealing the key.
3.  **Test Your Patterns**: Regex can be tricky. Test your patterns against dummy data to ensure they catch what you expect without redacting too much.
4.  **Use a Salt**: If using Hash Mode, always configure a secret salt in your local config to prevent reversing the hashes.