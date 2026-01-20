# Remote Profiles

Reglet can run compliance checks directly from remote profile URLs without downloading files manually.

## Basic Usage

```bash
# Run a remote profile
reglet check https://company.github.io/compliance/soc2.yaml

# Pin to a specific version (uses URL fragment)
reglet check https://example.com/profile.yaml#v1.2.0

# Pin to a specific content hash (immutable)
reglet check https://example.com/profile.yaml@sha256:abc123...

# Force re-fetch (bypass cache)
reglet check https://example.com/profile.yaml --refresh

# Trust a source without prompts
reglet check https://internal.example.com/profile.yaml --trust-source
```

## Profile Cache Management

Remote profiles are cached locally for 24 hours. Use these commands to manage the cache:

```bash
# List all cached profiles
reglet profile list

# Pre-fetch a profile without executing
reglet profile pull https://example.com/compliance.yaml

# Check for available updates
reglet profile outdated

# Clean up expired cached profiles
reglet profile prune

# Remove all cached profiles
reglet profile prune --all

# Preview what would be removed
reglet profile prune --dry-run
```

## Version Pinning

For reproducible CI/CD pipelines, pin profiles to specific versions:

### URL Fragment (`#version`)

```bash
# Pin to a tagged version
reglet check https://example.com/profile.yaml#v1.2.0
```

The version is recorded in `reglet.lock` for reproducibility.

### Content Hash (`@sha256:...`)

```bash
# Pin to an exact content hash (immutable)
reglet check https://example.com/profile.yaml@sha256:abc123def456...
```

Hash pinning ensures the exact same content is used every time.

## Trusted Sources

Configure pre-approved sources in `~/.reglet/config.yaml` to bypass interactive trust prompts:

```yaml
trusted_profile_sources:
  - "https://company.github.io/*"
  - "https://internal.example.com/profiles/*"
  - "https://*.trusted-domain.com/*"
```

**Glob patterns supported:**
- `*` matches any sequence of characters within a path segment
- Subdomain wildcards: `https://*.example.com/*`
- Path-specific patterns: `https://github.com/org/*/profiles/*`

## Security Features

Remote profiles are fetched with multiple security protections:

| Feature | Description |
|---------|-------------|
| **SSRF Protection** | Private IPs blocked by default. Use `--allow-private-network` to override. |
| **DNS Rebinding Protection** | Resolved IPs are pinned during fetch to prevent TOCTOU attacks. |
| **TLS Enforcement** | TLS 1.2+ required. Use `--insecure` to skip verification (not recommended). |
| **Credential Stripping** | Credentials in URLs (`user:pass@host`) are never logged or displayed. |
| **Size Limits** | Profiles limited to 10MB with streaming to prevent memory exhaustion. |
| **Path Traversal Protection** | Cache keys use SHA256 hashes, preventing path injection. |
| **Interactive Trust** | Prompts before executing untrusted remote profiles in TTY mode. |
| **Secret Detection** | Warns if fetched profiles contain hardcoded secrets (AWS keys, tokens, etc.). |

## CLI Flags

| Flag | Description |
|------|-------------|
| `--refresh` | Force re-fetch, bypassing cache |
| `--trust-source` | Trust this profile source for this run |
| `--insecure` | Skip TLS certificate verification |
| `--allow-private-network` | Allow fetching from private/internal IPs |
| `--fetch-timeout` | Timeout for HTTP requests (default: 30s) |

## Non-Interactive Mode (CI/CD)

In non-interactive environments (no TTY), remote profiles from untrusted sources will fail with an error message. Use one of these approaches:

1. **Pre-configure trusted sources** in `~/.reglet/config.yaml`
2. **Use `--trust-source` flag** to explicitly trust the source
3. **Pre-fetch with `reglet profile pull`** before running checks

Example CI/CD workflow:

```yaml
# .github/workflows/compliance.yml
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - name: Run compliance checks
        run: |
          reglet check https://company.github.io/soc2.yaml --trust-source
```

## Cache Location

Profiles are cached at `~/.reglet/profiles/<cache-key>/`:

```
~/.reglet/profiles/
├── a1b2c3d4e5f6.../
│   ├── profile.yaml    # Cached content
│   ├── metadata.json   # Fetch metadata (URL, etag, fetched_at)
│   └── digest.txt      # Content hash
└── ...
```

## Lockfile Integration

Remote profiles can be locked in `reglet.lock` for reproducible builds:

```yaml
# reglet.lock
version: 2
generated: 2024-01-19T12:00:00Z
profiles:
  "https://example.com/profile.yaml":
    requested: "https://example.com/profile.yaml#v1.2.0"
    resolved: "v1.2.0"
    digest: "sha256:abc123..."
    fetched: 2024-01-19T12:00:00Z
plugins:
  # ... plugin locks
```
