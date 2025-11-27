# Reglet Design Document

**Version**: 1.0
**Status**: Draft
**Last Updated**: November 2025

---

## Executive Summary

Reglet is a compliance and infrastructure validation platform that enables declarative policy definition, WASM-based extensibility, and OSCAL-compliant output. It supports standard compliance frameworks (ISO27001, SOC2, FedRAMP) as well as custom infrastructure validation rules.

The project follows an open-core model: a feature-rich open source CLI with enterprise features available commercially.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Goals & Non-Goals](#2-goals--non-goals)
3. [Core Concepts](#3-core-concepts)
4. [Configuration Format](#4-configuration-format)
5. [WASM Plugin System](#5-wasm-plugin-system)
6. [Execution Engine](#6-execution-engine)
7. [OSCAL Integration](#7-oscal-integration)
8. [CLI Interface](#8-cli-interface)
9. [Open Source Features](#9-open-source-features)
10. [Enterprise Features](#10-enterprise-features)
11. [Project Structure](#11-project-structure)
12. [Implementation Phases](#12-implementation-phases)
13. [Appendix: Examples](#13-appendix-examples)

---

## 1. Problem Statement

### Current Challenges

Organizations face significant friction in achieving and maintaining compliance:

- **Manual Evidence Collection**: Auditors require proof of compliance, but gathering evidence is time-consuming and error-prone.

- **Fragmented Tooling**: Different tools for different frameworks (ISO27001, SOC2, FedRAMP) with no unified approach.

- **Late Discovery**: Compliance issues found during audits rather than during development, making remediation expensive.

- **No Shift-Left**: Compliance validation happens after deployment rather than being integrated into CI/CD pipelines.

- **Rigid Scope**: Existing compliance tools don't support custom infrastructure validation (production readiness checks, service health gates).

- **Format Lock-in**: Results trapped in proprietary formats rather than open standards like OSCAL.

### Target Use Cases

1. **Continuous Compliance**: Automated validation against ISO27001, SOC2, FedRAMP, and other frameworks
2. **Audit Evidence Generation**: Point-in-time snapshots with cryptographic proof for auditors
3. **Infrastructure Validation**: Custom checks for production readiness, service health, deploy gates
4. **Shift-Left Compliance**: CI/CD integration to catch issues before deployment
5. **Internal Policy Enforcement**: Custom organizational standards beyond external frameworks

### Why Reglet? (vs OPA/Rego, Checkov, etc.)

Engineers will ask: "Why not use Open Policy Agent?" Here's the positioning:

**Reglet vs OPA/Rego:**

| Aspect | OPA/Rego | Reglet |
|--------|----------|------------|
| **Primary focus** | Kubernetes admission control, API authorization | Compliance validation, infrastructure checks, audit evidence |
| **Plugin language** | Rego (DSL, steep learning curve) | Go, Rust, TypeScript via WASM (languages teams know) |
| **Evidence collection** | Not a focus | First-class: every observation captures proof |
| **Output format** | JSON (custom schema) | OSCAL (compliance standard), SARIF, signed attestations |
| **Heavy computation** | Slow in Rego | Fast in WASM (native code) |
| **OS-level checks** | Requires external data push | Native: file contents, processes, services |
| **Network checks** | Not built-in | Native: HTTP, TCP, DNS, SMTP, certs |

**When to use each:**

| Use Case | Best Tool |
|----------|-----------|
| Kubernetes admission control | OPA/Gatekeeper |
| API authorization decisions | OPA |
| Real-time policy decisions (< 10ms) | OPA |
| Compliance audits with evidence | **Reglet** |
| Infrastructure validation (OS, network) | **Reglet** |
| CI/CD deploy gates with OSCAL output | **Reglet** |
| Cloud security posture (AWS, GCP) | **Reglet** |
| Custom checks in familiar languages | **Reglet** |

**Reglet vs Checkov/Trivy/ScoutSuite:**

| Aspect | Existing Tools | Reglet |
|--------|----------------|------------|
| **IaC scanning** | Strong (their focus) | Supported via plugins |
| **Runtime validation** | Limited | Strong (live infrastructure checks) |
| **Custom checks** | Python/YAML rules | WASM plugins (any language) |
| **Evidence for auditors** | Screenshots, exports | Native OSCAL, signed attestations |
| **Beyond security** | Security-focused | Compliance + operational validation |

**Our niche:** "Compliance for Developers"
- Write checks in languages you know (not Rego)
- Generate auditor-ready evidence automatically
- Validate live infrastructure, not just IaC
- OSCAL output for compliance automation
- Open core with path to enterprise

---

## 2. Goals & Non-Goals

### Goals

- **Declarative Profiles**: Define validation policies in human-readable YAML
- **WASM Extensibility**: Sandboxed plugins in any language that compiles to WASM
- **OSCAL Output**: Machine-readable compliance documents for interoperability
- **Evidence Collection**: Capture proof of compliance state for auditors
- **Shift-Left Integration**: CI/CD, pre-commit, IDE integration
- **Framework Agnostic**: Support any compliance framework via catalogs
- **Custom Validation**: Enable infrastructure validation beyond compliance
- **Open Core Model**: Generous open source with commercial enterprise features
- **Stateless Core**: No database or storage requirements; users control result persistence

### Non-Goals

- **Remediation Execution**: Reglet observes and reports; it does not modify systems (except optional `--fix` mode for simple cases)
- **Agent-Based Monitoring**: This is an agentless, pull-based system
- **Real-Time Alerting**: Core tool focuses on validation; alerting is an enterprise integration
- **SIEM Replacement**: Reglet feeds into existing security infrastructure, doesn't replace it
- **Built-in History (OSS)**: Historical trending and dashboards are enterprise features; OSS outputs results for users to store/compare as needed

---

## 3. Core Concepts

### OSCAL-Aligned Terminology

Reglet uses terminology aligned with NIST's Open Security Controls Assessment Language (OSCAL):

| Term | Definition |
|------|------------|
| **Catalog** | A compliance framework's control library (e.g., ISO27001, SOC2) |
| **Profile** | A selection of controls from catalogs forming a validation baseline |
| **Control** | A security or operational requirement to be validated |
| **Observation** | The *definition* of a check: which plugin to run, with what config, and what to expect |
| **Result** | The *outcome* of running an observation: status (passed/failed/error) + collected data |
| **Finding** | The conclusion about a control's compliance status (aggregated from results) |
| **Evidence** | Captured *artifacts* proving state: timestamps, raw data, hashes, screenshots |
| **Plugin** | A WASM module that executes observations and returns results |
| **POA&M** | Plan of Action & Milestones for addressing findings |

**Terminology flow:**
```
Observation (definition) → Plugin executes → Result (outcome) → Evidence (artifacts)
     ↓                                              ↓
  "Run HTTP check"                          "status: passed"
  "Expect 200"                              "response_time: 45ms"
```

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                       Profile File (YAML)                           │
│            Controls, Observations, Expectations                     │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Configuration Loader                           │
│  • Parse profile YAML                                               │
│  • Apply control defaults                                           │
│  • Resolve variables                                                │
│  • Load system config (plugin capabilities)                         │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Plugin Manager                               │
│  • Load WASM plugins (embedded → cache → registry)                  │
│  • Validate capability grants                                       │
│  • Initialize plugin instances                                      │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Execution Engine                               │
│  • Execute observations via plugins                                 │
│  • Evaluate expect expressions                                      │
│  • Collect evidence artifacts                                       │
│  • Aggregate findings                                               │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Output Formatter                               │
│  • Table (human-readable)                                           │
│  • JSON / YAML (machine-readable)                                   │
│  • SARIF (IDE/CI integration)                                       │
│  • OSCAL (compliance standard)                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. Configuration Format

### Profile Structure

Profiles define what to validate. They reference plugins and define controls with observations.

```yaml
profile:
  name: production-baseline
  version: 1.0.0
  description: Production environment compliance baseline

catalogs:                          # Optional - compliance framework references
  - iso27001
  - soc2-type2

plugins:                           # Plugin references (capabilities in system config)
  - reglet/file@1.0
  - reglet/http@1.0
  - reglet/smtp@1.0
  - ./plugins/custom-check.wasm

vars:
  environment: production
  api_host: api.example.com
  smtp_host: mail.example.com
  thresholds:
    response_time_ms:
      staging: 2000
      production: 500

controls:
  defaults:
    severity: warning
    owner: security-team
    tags: [production]
    # Retry configuration for transient failures
    retries: 3                         # Number of retry attempts (0 = no retries)
    retry_delay: 1s                    # Initial delay between retries
    retry_backoff: exponential         # none, linear, exponential
    retry_max_delay: 30s               # Cap for exponential backoff

  items:
    - id: ssh-root-disabled
      name: SSH Root Login Disabled
      description: Ensures root login via SSH is disabled
      severity: critical
      tags: [security, cis]           # Merges with defaults
      mappings:
        iso27001: A.9.4.2
        soc2: CC6.1
        cis: "5.2.10"

      observations:
        - plugin: file
          config:
            path: /etc/ssh/sshd_config
          expect:
            - exists == true
            - content contains "PermitRootLogin no"

      remediation: |
        Edit /etc/ssh/sshd_config and set:
        ```
        PermitRootLogin no
        ```
        Then restart sshd: `systemctl restart sshd`

    - id: api-health
      name: API Health Check
      description: Validates API is responding correctly
      # severity, owner inherited from defaults

      observations:
        - plugin: http
          config:
            url: "https://{{ .vars.api_host }}/health"
            timeout: 5s
          expect:
            - status_code == 200
            - response_time_ms < {{ index .vars.thresholds.response_time_ms .context }}
            - body contains "healthy"

    - id: smtp-ready
      name: SMTP Server Ready
      severity: critical
      tags: [email, infrastructure]

      observations:
        - plugin: smtp
          config:
            host: "{{ .vars.smtp_host }}"
            port: 587
          expect:
            - connected == true
            - supports_starttls == true

        - plugin: dns
          config:
            name: "{{ .vars.smtp_host }}"
            type: MX
          expect:
            - resolved == true

      remediation: |
        Ensure SMTP server supports STARTTLS and has valid MX records.
```

### Profile Inheritance

Profiles can extend other profiles using the `extends` field. This enables DRY configuration and organizational standards.

**Basic inheritance:**

```yaml
# base-security.yaml - organizational security baseline
profile:
  name: base-security
  version: 1.0.0

controls:
  defaults:
    severity: high
    owner: security-team

  items:
    - id: ssh-root-disabled
      name: SSH Root Login Disabled
      observations:
        - plugin: file
          config:
            path: /etc/ssh/sshd_config
          expect:
            - content contains "PermitRootLogin no"

    - id: firewall-enabled
      name: Firewall Active
      observations:
        - plugin: systemd
          config:
            unit: firewalld.service
          expect:
            - active == true
```

```yaml
# production.yaml - extends base with production-specific checks
profile:
  name: production
  version: 1.0.0

extends: ./base-security.yaml    # Inherit all controls from base

vars:
  api_host: api.prod.example.com

controls:
  defaults:
    tags: [production]           # Merged with parent defaults

  items:
    # Additional production-only controls
    - id: api-health
      name: Production API Health
      severity: critical
      observations:
        - plugin: http
          config:
            url: "https://{{ .vars.api_host }}/health"
          expect:
            - status_code == 200

    # Override parent control (same ID replaces)
    - id: ssh-root-disabled
      name: SSH Root Login Disabled (Strict)
      severity: critical         # Upgraded from high
      observations:
        - plugin: file
          config:
            path: /etc/ssh/sshd_config
          expect:
            - content contains "PermitRootLogin no"
            - content contains "PasswordAuthentication no"  # Additional check
```

**Merge behavior:**

| Element | Behavior |
|---------|----------|
| `profile.*` | Child overrides parent |
| `vars` | Deep merge (child wins on conflict) |
| `plugins` | Concatenated (deduplicated) |
| `catalogs` | Concatenated (deduplicated) |
| `controls.defaults` | Deep merge (child wins on conflict) |
| `controls.items` | By ID: same ID = replace, new ID = append |

**Multiple inheritance:**

```yaml
profile:
  name: production-pci
  version: 1.0.0

extends:
  - ./base-security.yaml        # First parent
  - ./pci-dss-baseline.yaml     # Second parent (wins on conflict)

# Resolution order: base-security → pci-dss-baseline → this file
```

**Inheritance rules:**
1. Profiles are resolved depth-first (base parents first)
2. Later extends win on conflict (last-write-wins)
3. Circular dependencies are detected and rejected at validation time
4. Relative paths are resolved from the extending profile's directory
5. Remote URLs supported: `extends: https://example.com/profiles/base.yaml`

**Viewing resolved profile:**

```bash
# See the fully merged profile
reglet plan production.yaml --show-resolved

# Output shows all inherited controls with their source
# [base-security.yaml] ssh-root-disabled
# [production.yaml] ssh-root-disabled (override)
# [base-security.yaml] firewall-enabled
# [production.yaml] api-health
```

> **`extends:` vs `imports:`:**
> - `extends:` - Full profile inheritance. Merges `profile.*`, `vars`, `controls.defaults`, and `controls.items`. Use for organizational baselines where child profiles customize parent settings.
> - `imports:` - Control-only inclusion. Only imports `controls.items` from the referenced files. No profile metadata, vars, or defaults are inherited. Use for modular control libraries (see Phase 8: Compliance Packs Registry).

### System Configuration

Plugin capabilities are managed separately in system config, not in profiles. This ensures profiles are safe to share and can't escalate permissions.

```yaml
# ~/.reglet/config.yaml

# Plugin capability grants
plugins:
  # Built-in plugins
  reglet/file@1.0:
    capabilities:
      - fs:read:/etc/**
      - fs:read:/var/log/**

  reglet/http@1.0:
    capabilities:
      - network:outbound:80,443

  reglet/smtp@1.0:
    capabilities:
      - network:outbound:25,587,465

  reglet/aws@1.0:
    capabilities:
      - network:outbound
      - env:AWS_*
    auth: oidc:aws                    # Use OIDC exchange (preferred in CI)
    # Falls back to env:AWS_* if OIDC not available

  # Custom plugins
  ./plugins/custom-check.wasm:
    capabilities:
      - fs:read:/app/config/**
      - exec:systemctl
    secrets:                         # Whitelist secrets this plugin can access
      - custom_api_key

# Secrets management
secrets:
  # Simple secrets (local/dev) - stored directly in config
  local:
    dev_api_key: "test-key-123"

  # Environment variable references
  env:
    aws_access_key: AWS_ACCESS_KEY_ID
    aws_secret_key: AWS_SECRET_ACCESS_KEY

  # File references (read at runtime)
  files:
    tls_cert: /etc/ssl/certs/app.pem
    tls_key: /etc/ssl/private/app.key
    db_password: /run/secrets/db_password

  # External secrets manager (Enterprise)
  # vault:
  #   address: https://vault.example.com
  #   auth: kubernetes
  #   mappings:
  #     prod_api_key: secret/data/prod#api_key
  #
  # aws-secrets-manager:
  #   region: us-east-1
  #   mappings:
  #     prod_db_password: prod/database/password

# OIDC Authentication (for CI/CD pipelines)
# Eliminates long-lived API keys in favor of short-lived tokens
oidc:
  providers:
    # GitHub Actions OIDC
    github-actions:
      token_env: ACTIONS_ID_TOKEN_REQUEST_TOKEN
      url_env: ACTIONS_ID_TOKEN_REQUEST_URL

    # GitLab CI OIDC
    gitlab-ci:
      token_env: CI_JOB_JWT_V2

    # Generic OIDC (custom IdP)
    # custom:
    #   token_env: MY_OIDC_TOKEN
    #   issuer: https://idp.example.com

  # Token exchanges - trade CI tokens for cloud credentials
  exchanges:
    aws:
      provider: github-actions          # Which OIDC provider to use
      role_arn: arn:aws:iam::123456789012:role/reglet-ci
      session_duration: 1h              # Optional, default 1h
      # Automatically calls STS AssumeRoleWithWebIdentity

    gcp:
      provider: github-actions
      workload_identity_provider: projects/123456/locations/global/workloadIdentityPools/github/providers/github
      service_account: reglet@project.iam.gserviceaccount.com
      # Automatically exchanges for GCP access token

    azure:
      provider: github-actions
      tenant_id: 00000000-0000-0000-0000-000000000000
      client_id: 00000000-0000-0000-0000-000000000001
      # Uses federated credentials, no client secret needed

# Default context if not specified
defaults:
  context: production

# Catalog search paths
catalogs:
  paths:
    - ~/.reglet/catalogs
    - /etc/reglet/catalogs

# Plugin and Pack registries
registries:
  # Default public registry (plugins + community packs)
  public:
    url: https://registry.reglet.dev
    # No auth required for public registry

  # Enterprise/Private registry (certified packs, org-specific)
  enterprise:
    url: https://private.reglet.io
    auth:
      username: "license-id"
      # Token from 'reglet login' or environment
      token_env: REGLET_ENTERPRISE_TOKEN

  # Self-hosted registry (air-gapped environments)
  # internal:
  #   url: https://reglet.internal.corp
  #   auth:
  #     token_env: INTERNAL_REGISTRY_TOKEN
  #   tls:
  #     ca_cert: /etc/ssl/certs/internal-ca.pem

# Registry resolution order (first match wins)
registry_order:
  - enterprise    # Check private first
  - public        # Fall back to public

# Output preferences
output:
  format: table
  color: auto
```

### Variable Resolution

Variables support Go template syntax with built-in functions:

```yaml
vars:
  base_url: https://api.example.com
  timeout:
    staging: 30
    production: 10

controls:
  items:
    - id: api-check
      observations:
        - plugin: http
          config:
            url: "{{ .vars.base_url }}/health"
            timeout: "{{ index .vars.timeout .context }}s"
```

**Available template functions:**
- `ternary`: Conditional - `{{ ternary .condition "yes" "no" }}`
- `default`: Default value - `{{ default .vars.timeout 30 }}`
- `index`: Map access - `{{ index .vars.thresholds .context }}`
- `upper`, `lower`, `title`: Case conversion
- `trim`, `replace`, `contains`: String manipulation
- `env`: Environment variable - `{{ env "API_KEY" }}`

### Secrets in Profiles

Profiles reference secrets by name. Values are resolved at runtime from system config and never stored in profiles.

```yaml
# profile.yaml
controls:
  items:
    - id: external-api-check
      observations:
        - plugin: http
          config:
            url: https://api.example.com/status
            headers:
              Authorization: "Bearer {{ .secrets.api_token }}"

        - plugin: custom-plugin
          config:
            api_key: "{{ .secrets.custom_api_key }}"
            endpoint: https://service.example.com
```

**Security model:**
- Profiles only contain secret *names*, not values
- Plugins must whitelist which secrets they can access
- System config file should have restricted permissions (0600)
- Secrets from external managers (Vault, AWS SM) are enterprise features

**Edge cases and error handling:**

| Scenario | Behavior |
|----------|----------|
| Secret referenced but not defined | **Error** - fail fast with clear message |
| Secret defined but plugin not whitelisted | **Error** - capability violation |
| Secret source unavailable (file missing) | **Error** - fail at startup, not mid-run |
| Empty secret value | **Warning** - allowed but logged |

```bash
# Missing secret example
$ reglet check profile.yaml
Error: Secret 'api_token' referenced in control 'external-api-check'
       but not defined in system config.

       Add to ~/.reglet/config.yaml:
         secrets:
           env:
             api_token: API_TOKEN_ENV_VAR
```

**Secret resolution timing:**
- All secrets are resolved **once** at profile load time
- Long-running checks do NOT see rotated secrets mid-execution
- For secret rotation, re-run the check after rotation completes

**Redaction and hashing:**

When `hash_mode` is enabled for evidence redaction:
- Hashes use SHA-256 truncated to first 8 characters
- Hashes are for **correlation only** (prove same value across runs)
- Truncation prevents rainbow table attacks
- Timing-safe comparison used internally (constant-time)

```yaml
# Evidence output with hash_mode enabled
evidence:
  data:
    api_key: "[API_KEY:sha256:7f3d9a2b]"  # Truncated hash
    # Same value always produces same hash prefix
    # Different values produce different prefixes
```

**⚠️ Hash security note:**
Hashes of low-entropy secrets (short passwords, sequential IDs) may still be vulnerable. For maximum security, use `--redact-full` which replaces with `[REDACTED]` without any hash.

### CI/CD OIDC Authentication

Modern CI/CD systems (GitHub Actions, GitLab CI) support OpenID Connect (OIDC) for federated authentication. Instead of storing long-lived API keys as secrets, the CI runner requests a short-lived JWT token that can be exchanged for cloud credentials.

**Why OIDC matters:**
- **No long-lived secrets**: Tokens live for minutes, not years
- **No secret rotation**: No credentials to rotate or leak
- **Audit trail**: Cloud providers log which CI job requested credentials
- **Least privilege**: Scope credentials to specific repos/branches

**How it works in Reglet:**

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         OIDC Token Exchange Flow                              │
└──────────────────────────────────────────────────────────────────────────────┘

  ┌─────────────┐         ┌─────────────┐         ┌─────────────┐
  │   GitHub    │         │   Reglet    │         │  AWS STS    │
  │   Actions   │         │     CLI     │         │             │
  └──────┬──────┘         └──────┬──────┘         └──────┬──────┘
         │                       │                       │
         │  ① CI job starts      │                       │
         │  Sets env vars:       │                       │
         │  ACTIONS_ID_TOKEN_... │                       │
         │ ─────────────────────▶│                       │
         │                       │                       │
         │      ┌────────────────┴────────────────┐      │
         │      │ ② Reglet detects CI env        │      │
         │      │    Reads OIDC token from env   │      │
         │      │                                 │      │
         │      │    JWT Token contains:          │      │
         │      │    {                            │      │
         │      │      "iss": "github.com",       │      │
         │      │      "sub": "repo:org/repo:*",  │      │
         │      │      "aud": "sts.amazonaws.com",│      │
         │      │      "exp": 1705312200          │      │
         │      │    }                            │      │
         │      └────────────────┬────────────────┘      │
         │                       │                       │
         │                       │  ③ AssumeRoleWithWebIdentity
         │                       │     RoleArn: arn:aws:iam::...
         │                       │     WebIdentityToken: <JWT>
         │                       │ ─────────────────────▶│
         │                       │                       │
         │                       │      ┌────────────────┴──────┐
         │                       │      │ ④ AWS validates:      │
         │                       │      │   - JWT signature     │
         │                       │      │   - Issuer (github)   │
         │                       │      │   - Audience (sts)    │
         │                       │      │   - Subject (repo)    │
         │                       │      │   - Expiration        │
         │                       │      │   - Trust policy match│
         │                       │      └────────────────┬──────┘
         │                       │                       │
         │                       │  ⑤ Temporary credentials
         │                       │     AccessKeyId: ASIA...
         │                       │     SecretAccessKey: ...
         │                       │     SessionToken: ...
         │                       │     Expiration: +1 hour
         │                       │ ◀─────────────────────│
         │                       │                       │
         │      ┌────────────────┴────────────────┐      │
         │      │ ⑥ Reglet uses temp creds       │      │
         │      │    to call AWS APIs            │      │
         │      │    (S3, EC2, IAM, etc.)        │      │
         │      └────────────────┬────────────────┘      │
         │                       │                       │
         │                       │  ⑦ AWS API calls      │
         │                       │ ─────────────────────▶│
         │                       │                       │
         │                       │  ⑧ Results            │
         │                       │ ◀─────────────────────│
         │                       │                       │
```

**The key insight:** Your CI platform (GitHub/GitLab) acts as an identity provider. AWS/GCP/Azure trust that identity provider. No secrets ever stored in CI - just a trust relationship.

**GitHub Actions example:**

```yaml
# .github/workflows/compliance.yaml
name: Compliance Check
on: [push]

permissions:
  id-token: write    # Required for OIDC
  contents: read

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Reglet
        uses: reglet/action@v1
        # No AWS credentials needed! OIDC handles it
        # The action automatically detects GITHUB_ACTIONS=true
        # and uses ACTIONS_ID_TOKEN_REQUEST_TOKEN
```

**GitLab CI example:**

```yaml
# .gitlab-ci.yml
compliance:
  image: reglet/cli:latest
  id_tokens:
    GITLAB_OIDC_TOKEN:
      aud: https://reglet.dev  # Or your AWS/GCP audience
  script:
    - reglet check profile.yaml
  # No CI_JOB_TOKEN or AWS keys needed
```

**Automatic detection:**

Reglet automatically detects CI environments and enables OIDC:

| Environment | Detection | Token Source |
|-------------|-----------|--------------|
| GitHub Actions | `GITHUB_ACTIONS=true` | `ACTIONS_ID_TOKEN_REQUEST_TOKEN` |
| GitLab CI | `GITLAB_CI=true` | `CI_JOB_JWT_V2` or `id_tokens` |
| CircleCI | `CIRCLECI=true` | `CIRCLE_OIDC_TOKEN` |
| Buildkite | `BUILDKITE=true` | `BUILDKITE_OIDC_TOKEN` |

**Fallback behavior:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Authentication Flow                           │
└─────────────────────────────────────────────────────────────────┘

1. Is OIDC token available?
   │
   ├─▶ YES: Attempt OIDC exchange
   │   │
   │   ├─▶ Exchange succeeds → Use temporary credentials ✓
   │   │
   │   └─▶ Exchange fails → ERROR (do NOT fall back silently)
   │       │
   │       └─▶ Security: Silent fallback could expose static creds
   │           that were meant to be unused
   │
   └─▶ NO: Use static credentials
       │
       └─▶ Check in order:
           a. auth: config in system config
           b. env: mappings (AWS_ACCESS_KEY_ID, etc.)
           c. Default credential chain (AWS SDK, gcloud, etc.)
```

**Why OIDC exchange failure is an error (not fallback):**

| Scenario | Behavior | Rationale |
|----------|----------|-----------|
| OIDC token unavailable | Fall back to static | Expected in non-CI environments |
| OIDC exchange fails (network) | **Error** | May be transient, retry manually |
| OIDC exchange fails (role) | **Error** | Misconfiguration, fix IAM/trust policy |
| OIDC exchange fails (expired) | **Error** | CI job running too long |

```bash
# OIDC exchange failure example
$ reglet check profile.yaml
Error: OIDC token exchange failed

  Provider: GitHub Actions
  Exchange: AWS STS AssumeRoleWithWebIdentity
  Error: AccessDenied - Not authorized to perform sts:AssumeRoleWithWebIdentity

  Possible causes:
    - IAM role trust policy doesn't allow this repository
    - Audience mismatch in trust policy
    - OIDC provider not configured in AWS

  To skip OIDC and use static credentials:
    reglet check profile.yaml --auth-static

  To require OIDC (fail if unavailable):
    reglet check profile.yaml --auth-oidc-required
```

**CLI flags for OIDC:**

```bash
# Force OIDC even if not auto-detected
reglet check profile.yaml --auth-oidc

# Force skip OIDC (use static credentials)
reglet check profile.yaml --auth-static

# Require OIDC - fail if not available (recommended for CI)
reglet check profile.yaml --auth-oidc-required
# Error: OIDC required but no token available
#        Run in a CI environment with OIDC support, or remove --auth-oidc-required

# Debug authentication
reglet check profile.yaml --verbose
# [auth] Detected: GitHub Actions
# [auth] Requesting OIDC token for audience: sts.amazonaws.com
# [auth] Exchanging token for AWS credentials (role: arn:aws:iam::123...)
# [auth] Temporary credentials valid for 1h
```

**AWS IAM role trust policy:**

For OIDC to work, your cloud provider needs a trust policy. Here's an AWS example:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:myorg/myrepo:*"
        }
      }
    }
  ]
}
```

### Profile Schema Validation

Profiles are validated against a JSON Schema before execution. This catches configuration errors early.

**Schema location:**
- Published at `https://reglet.dev/schemas/profile.schema.json`
- Bundled in CLI binary for offline validation
- VS Code extension provides real-time validation

**Validation with `reglet plan`:**

```bash
# Full validation without execution
reglet plan profile.yaml --validate-only

# Output on success:
# ✓ Profile syntax valid (YAML parseable)
# ✓ Schema validation passed
# ✓ All plugins available (embedded: 3, cached: 1)
# ✓ All control IDs unique
# ✓ No circular dependencies
# ✓ All referenced variables defined
# ✓ All template expressions valid
# ✓ All secret references have corresponding grants

# Output on failure:
# ✗ Schema validation failed:
#   - controls.items[2].observations[0]: missing required field 'plugin'
#   - controls.items[5].severity: must be one of: critical, high, medium, low
#   - vars.timeout: expected object, got string
```

**Key schema rules:**

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `profile.name` | string | yes | 1-64 chars, alphanumeric + hyphens |
| `profile.version` | string | yes | semver format (x.y.z) |
| `controls.items[].id` | string | yes | unique within profile |
| `controls.items[].severity` | enum | no | critical, high, medium, low, info |
| `observations[].plugin` | string | yes | valid plugin reference |
| `observations[].expect` | array | no | valid expr-lang expressions |
| `observations[].timeout` | duration | no | format: 1s, 5m, 1h |

**IDE integration:**

```yaml
# Add to top of profile for VS Code/JetBrains validation
# yaml-language-server: $schema=https://reglet.dev/schemas/profile.schema.json

profile:
  name: my-profile
  # ... IDE now provides autocomplete and validation
```

**Programmatic validation:**

```bash
# Validate without any execution planning
reglet validate profile.yaml

# Exit codes:
# 0 = valid
# 1 = invalid (schema errors)
# 2 = file not found or unparseable
```

> **`reglet validate` vs `reglet plan --validate-only`:**
> - `reglet validate` - Fast syntax/schema check only. Doesn't resolve plugins or build execution graph. Use for IDE integration and pre-commit hooks.
> - `reglet plan --validate-only` - Full validation including plugin availability, dependency resolution, and expression parsing. Use before CI runs to catch all issues.

---

## 5. WASM Plugin System

### Overview

Plugins are WebAssembly modules that perform observations. They run in a sandboxed environment with explicit capability grants.

### Plugin Types

**Capability Plugins**: Provide observation primitives
- Execute specific checks (file existence, HTTP requests, etc.)
- Return structured evidence data
- Examples: `file`, `http`, `tcp`, `dns`, `smtp`, `command`

**Control Bundles**: Package complete controls with observations
- Include control definitions + observation logic
- Useful for distributing complete compliance checks
- Example: `cis-linux-benchmark` bundle with all CIS controls

### Capability System

Plugins declare required capabilities; system config grants them:

**Capability Categories:**

| Category | Pattern | Description |
|----------|---------|-------------|
| Filesystem | `fs:read:<glob>` | Read files matching pattern |
| Filesystem | `fs:write:<glob>` | Write files matching pattern |
| Network | `network:outbound` | Make any outbound connections |
| Network | `network:outbound:<ports>` | Outbound to specific ports (e.g., `53`, `80,443`, `8000-9000`) |
| Environment | `env:<pattern>` | Read environment variables |
| Execute | `exec:<commands>` | Run specific commands |

**Capability Enforcement:**
- Plugin declares what it needs in its manifest
- System config grants (or denies) capabilities
- Runtime enforces grants - unauthorized access fails immediately
- Profiles cannot grant capabilities (security boundary)

### Host Functions for Network Operations

**Challenge**: WASI Preview 1 (the current stable WASI standard) does not include socket support, making direct network operations from WASM impossible. WASI Preview 2 will address this, but it's not yet stable or widely supported.

**Solution**: Host functions bridge the gap by allowing WASM plugins to call native Go functions for network operations while maintaining the capability-based security model.

**Architecture**:
```
┌─────────────────────────────────────────────────┐
│ WASM Plugin (Sandboxed)                         │
│                                                  │
│  //go:wasmimport reglet_host dns_lookup         │
│  //go:wasmimport reglet_host http_request       │
│  //go:wasmimport reglet_host tcp_connect        │
│                                                  │
│  func observe() {                                │
│    result = host_dns_lookup(hostname, type)     │
│    return result                                 │
│  }                                               │
└──────────────────┬──────────────────────────────┘
                   │ Linear Memory (shared)
                   │ JSON data exchange
                   ▼
┌─────────────────────────────────────────────────┐
│ Host Functions (Native Go)                      │
│                                                  │
│  func DNSLookup() {                              │
│    1. Check capability grant                    │
│    2. Read params from WASM memory              │
│    3. Perform actual network operation          │
│    4. Write result to WASM memory               │
│  }                                               │
└─────────────────────────────────────────────────┘
```

**Available Host Functions**:

| Function | Capability Required | Purpose |
|----------|-------------------|---------|
| `dns_lookup` | `network:outbound:53` | DNS resolution (A, AAAA, CNAME, MX, TXT, NS) |
| `http_request` | `network:outbound:80,443` | HTTP/HTTPS requests (GET, POST, etc.) |
| `tcp_connect` | `network:outbound:<port>` | TCP/TLS connection testing |

**Security Model**:
- Host functions check capability grants **before** performing operations
- Pattern matching enforces granular permissions:
  - `outbound:53` - Only DNS (port 53)
  - `outbound:80,443` - Only HTTP/HTTPS (port list)
  - `outbound:8000-9000` - Development ports (port range)
  - `outbound:*` - Any port (use sparingly)
- Operations without matching grants fail with "capability denied" error
- WASM sandbox + capability enforcement = defense in depth

**Memory Management**:
- Plugins allocate memory in WASM linear memory
- Host reads/writes data at specified pointers
- JSON used for structured data exchange
- Plugins must deallocate memory after use

**Example Plugin Usage**:
```go
// Plugin side (WASM)
//go:wasmimport reglet_host dns_lookup
func host_dns_lookup(hostnamePtr, hostnameLen, typePtr, typeLen uint32) uint32

func observe(configPtr, configLen uint32) uint32 {
    // Allocate and copy hostname to memory
    hostnamePtr := allocate(len(hostname))
    copyToMemory(hostnamePtr, []byte(hostname))

    // Call host function
    resultPtr := host_dns_lookup(hostnamePtr, len(hostname), typePtr, len(recordType))

    // Read result from memory
    result := readFromMemory(resultPtr, maxSize)
    return marshalToPtr(result)
}
```

**Built-in Plugins Using Host Functions**:
- `dns` - DNS lookup and validation
- `http` - HTTP endpoint checking
- `tcp` - TCP/TLS connection testing
- `smtp` - Email server testing (future)
- `cert` - Certificate validation (future)

This approach maintains the WASM sandbox architecture while enabling network operations until WASI Preview 2 stabilizes.

### Plugin Integrity & Supply Chain Security

Plugins are executable code. Beyond sandboxing, we must verify plugin artifacts haven't been tampered with.

**Lockfile (Phase 1b - OSS):**

The lockfile pins **exact versions** and **checksums** for deterministic, reproducible builds. Profile references like `@1.0` are resolved once and locked.

```yaml
# profile.yaml (human-authored, flexible version specs)
plugins:
  - reglet/aws@1.0      # Could resolve to 1.0.0, 1.0.1, 1.0.2...
  - reglet/http@^1.2    # Any 1.x >= 1.2
  - reglet/dns@1.0.3    # Exact version
```

```yaml
# reglet.lock (auto-generated, committed to repo)
# This file is the source of truth for CI/CD - deterministic builds guaranteed
lockfile_version: 1

plugins:
  reglet/aws:
    requested: "@1.0"                    # What profile asked for
    resolved: "1.0.2"                    # Exact version locked
    source: https://plugins.reglet.dev/aws/1.0.2/aws.wasm
    sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
    fetched: 2025-01-15T10:30:00Z

  reglet/http:
    requested: "@^1.2"
    resolved: "1.3.1"
    source: https://plugins.reglet.dev/http/1.3.1/http.wasm
    sha256: 7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730
    fetched: 2025-01-15T10:30:00Z

  reglet/dns:
    requested: "@1.0.3"
    resolved: "1.0.3"
    source: https://plugins.reglet.dev/dns/1.0.3/dns.wasm
    sha256: a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456
    fetched: 2025-01-15T10:30:00Z

  ./plugins/custom.wasm:
    requested: "local"
    resolved: "local"
    source: ./plugins/custom.wasm
    sha256: 60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752
    modified: 2025-01-14T15:22:00Z
```

**Version resolution rules:**
| Profile Syntax | Meaning | Lockfile Behavior |
|----------------|---------|-------------------|
| `@1.0` | Latest 1.0.x | Locks to exact (e.g., 1.0.2) |
| `@^1.2` | Any 1.x >= 1.2 | Locks to exact (e.g., 1.3.1) |
| `@~1.2.3` | Any 1.2.x >= 1.2.3 | Locks to exact (e.g., 1.2.5) |
| `@1.2.3` | Exactly 1.2.3 | Locks to 1.2.3 |
| `@latest` | Latest stable | Locks to exact at fetch time |

**Behavior:**
```bash
# First run (no lockfile): resolves versions, downloads, generates lockfile
reglet check profile.yaml
# Resolving plugins...
#   reglet/aws@1.0 → 1.0.2
#   reglet/http@^1.2 → 1.3.1
# Created reglet.lock
# Commit this file for reproducible builds.

# Subsequent runs: uses locked versions, verifies checksums
reglet check profile.yaml
# Using locked versions from reglet.lock
# ✓ reglet/aws@1.0.2 verified
# ✓ reglet/http@1.3.1 verified

# If plugin author pushes new 1.0.3, you still get 1.0.2 (locked)
# CI pipelines are deterministic and won't break unexpectedly

# If checksum doesn't match (tampering or registry corruption):
# ✗ Plugin reglet/aws@1.0.2 checksum mismatch!
#   Expected: 9f86d081...
#   Got: 3c9909af...
#   This could indicate tampering. Investigate before updating.

# Explicitly update to get newer versions
reglet plugin update                    # Update all plugins
reglet plugin update reglet/aws     # Update specific plugin
# Review changes in reglet.lock before committing
```

**Signing (Phase 2 - Official Plugins):**

Official plugins are signed using Sigstore/Cosign. The CLI verifies signatures before execution.

```bash
# Install with signature verification (default for registry plugins)
reglet plugin install reglet/aws@1.0

# Verifying signature...
# ✓ Signed by: reglet-release@reglet.dev
# ✓ Transparency log entry found
# ✓ Plugin installed

# Skip verification (local/dev only)
reglet plugin install ./my-plugin.wasm --skip-verify
```

**Trust model:**
| Plugin Source | Checksum (Phase 1b) | Signature (Phase 2) |
|---------------|-------------------|---------------------|
| Official registry | Required | Required |
| Community registry | Required | Optional (if signed) |
| Local file | Required | Skipped |

### Plugin Interface (WIT)

Plugins implement a standard interface defined in WebAssembly Interface Types (WIT):

```wit
// reglet.wit

package reglet:plugin@1.0.0;

interface types {
    record config {
        values: list<tuple<string, string>>,
    }

    record evidence {
        timestamp: string,
        data: list<tuple<string, value>>,
        raw: option<string>,
    }

    variant value {
        string(string),
        int(s64),
        float(f64),
        bool(bool),
        list(list<value>),
    }

    record error {
        code: string,
        message: string,
    }

    record capability {
        kind: string,
        pattern: string,
    }

    record plugin-info {
        name: string,
        version: string,
        description: string,
        capabilities: list<capability>,
    }

    record config-schema {
        fields: list<field-def>,
    }

    record field-def {
        name: string,
        field-type: string,
        required: bool,
        description: string,
    }
}

interface plugin {
    use types.{config, evidence, error, plugin-info, config-schema};

    // Return plugin metadata
    describe: func() -> plugin-info;

    // Return configuration schema
    schema: func() -> config-schema;

    // Execute observation and return evidence
    observe: func(config: config) -> result<evidence, error>;
}

world reglet-plugin {
    export plugin;
}
```

### Plugin Development

**Scaffolding:**
```bash
$ reglet plugin init my-plugin --lang rust
Creating plugin scaffold in ./my-plugin
  my-plugin/
  ├── Cargo.toml
  ├── src/
  │   └── lib.rs
  ├── wit/
  │   └── reglet.wit
  └── README.md
```

**Testing:**
```bash
$ reglet plugin test ./my-plugin
Running plugin tests...
✓ describe() returns valid plugin info
✓ schema() returns valid config schema
✓ observe() with valid config returns evidence
✓ observe() with invalid config returns error
All tests passed
```

**Building:**
```bash
$ reglet plugin build ./my-plugin
Compiling my-plugin to WASM...
Output: ./my-plugin/target/my-plugin.wasm
```

**Installing:**
```bash
$ reglet plugin install ./my-plugin/target/my-plugin.wasm

Plugin: my-plugin@1.0.0
Description: Custom validation plugin
Requests capabilities:
  - fs:read:/app/config/** (read application config files)
  - network:outbound:443 (make HTTPS requests)

Allow these capabilities? [y/N] y
✓ Plugin installed to ~/.reglet/plugins/my-plugin@1.0.0.wasm
✓ Capabilities added to ~/.reglet/config.yaml
```

### Built-in Plugins

| Plugin | Description | Key Capabilities |
|--------|-------------|------------------|
| `file` | File existence, permissions, content | `fs:read` |
| `command` | Execute commands, validate output | `exec` |
| `http` | HTTP/HTTPS requests | `network:outbound` |
| `tcp` | TCP connectivity, TLS validation | `network:outbound` |
| `dns` | DNS resolution | `network:outbound:53` |
| `smtp` | SMTP connectivity, STARTTLS | `network:outbound:25,587,465` |
| `cert` | Certificate validation | `network:outbound` |
| `process` | Running process checks | `fs:read:/proc` |
| `systemd` | Service status | `exec:systemctl` |
| `secrets` | Secret detection in files | `fs:read` |
| `container` | Container image scanning | `network:outbound` |

### Plugin Distribution

**Batteries Included**: The `reglet` binary ships with core plugins embedded. No downloads required for basic usage.

```go
// internal/plugins/embedded.go
import "embed"

//go:embed wasm/*.wasm
var embeddedPlugins embed.FS
```

**Resolution order:**

```
1. Embedded plugins (built into binary)     ← Zero setup, always available
   └─▶ file, command, http, tcp, dns, smtp, cert, process, systemd

2. Local cache (~/.reglet/plugins/)     ← Previously downloaded
   └─▶ Versioned: aws@1.0.2.wasm, gcp@2.1.0.wasm

3. Plugin registry (plugins.reglet.dev) ← On-demand download
   └─▶ Only when profile references non-embedded plugin
```

**Why this matters:**

```bash
# Fresh install - works immediately, no network required
curl -fsSL https://reglet.dev/install.sh | sh
reglet check my-profile.yaml  # ✓ Just works

# Embedded plugins don't need version specifiers
plugins:
  - file      # Uses embedded (always latest in this binary)
  - http      # Uses embedded
  - aws@1.0   # Downloads from registry (not embedded)
```

**Version behavior:**

| Reference | Resolution |
|-----------|------------|
| `file` | Embedded version (no download) |
| `file@1.0` | Embedded if compatible, else download |
| `file@2.0` | Download (if embedded is 1.x) |
| `aws@1.0` | Download (not embedded) |
| `./custom.wasm` | Local file |

**Listing plugins:**

```bash
$ reglet plugin list

EMBEDDED (built-in):
  file        1.0.0   File existence, permissions, content
  command     1.0.0   Execute commands, validate output
  http        1.0.0   HTTP/HTTPS requests
  tcp         1.0.0   TCP connectivity, TLS validation
  dns         1.0.0   DNS resolution
  smtp        1.0.0   SMTP connectivity, STARTTLS
  cert        1.0.0   Certificate validation
  process     1.0.0   Running process checks
  systemd     1.0.0   Service status

CACHED (~/.reglet/plugins/):
  aws         1.0.2   AWS resource validation
  gcp         2.1.0   GCP resource validation

AVAILABLE (registry):
  Run 'reglet plugin search <query>' to find more plugins
```

**Offline mode:**

```bash
# Works completely offline with embedded plugins
reglet check profile.yaml --offline

# Fails if profile needs non-embedded plugins
# Error: Plugin 'aws@1.0' not in cache. Run without --offline to download.
```

---

## 6. Execution Engine

### Execution Flow

1. **Load Profile**: Parse YAML, apply defaults, resolve variables
2. **Load Plugins**: Initialize WASM runtime, load plugins with granted capabilities
3. **Pre-flight Validation**: Validate observation configs against plugin schemas (fail fast)
4. **Build Execution Plan**: Resolve control dependencies, build DAG
5. **Execute Controls**: Run observations in parallel by dependency level
6. **Evaluate Expectations**: Check expect expressions against evidence
7. **Generate Findings**: Determine pass/fail status for each control
8. **Format Output**: Render results in requested format

### Pre-flight Validation

Before executing any observations, the engine validates all plugin configurations against their declared schemas. This catches configuration errors early, before any side effects occur.

**Why this matters:**
- A typo in `config.path` shouldn't cause a runtime panic 5 minutes into execution
- Type mismatches (string vs int) are caught immediately with clear error messages
- Missing required fields fail at validation, not deep in plugin code

**Validation sequence:**

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│ Load Plugin │────▶│ Call schema()│────▶│ Validate config │
└─────────────┘     └──────────────┘     └─────────────────┘
                           │                      │
                           ▼                      ▼
                    ┌─────────────┐        ┌─────────────┐
                    │ JSON Schema │        │ Pass / Fail │
                    └─────────────┘        └─────────────┘
```

**For each observation in the profile:**
1. Look up the plugin's `schema()` export (cached after first call)
2. Validate `observation.config` against the returned JSON Schema
3. Collect all errors (don't stop at first)
4. If any validation fails, abort with combined error report

**Example validation error output:**

```
$ reglet check profile.yaml

Pre-flight validation failed (3 errors):

  controls.items[0].observations[0] (plugin: file):
    - config.path: required field missing

  controls.items[2].observations[1] (plugin: http):
    - config.timeout: expected integer, got string "30s"
    - config.method: must be one of: GET, POST, PUT, DELETE

  controls.items[5].observations[0] (plugin: tcp):
    - config.port: must be between 1 and 65535, got 0

Run 'reglet plan --validate-only' for full validation without execution.
```

**Schema caching:**
- Plugin schemas are cached in memory after first `schema()` call
- Schemas are immutable for a given plugin version
- Cache is invalidated on plugin update

### Parallel Execution Model

The engine executes controls in parallel using a **leveled topological sort**. Controls are grouped into N levels based on their dependency depth, and all controls at each level execute concurrently.

```
Level 0 (no dependencies):     [A] [B] [C]     ← Execute in parallel
                                ↓   ↓   ↓
Level 1:                       [D] [E]         ← Execute in parallel after L0
                                ↓   ↓
Level 2:                       [F] [G]         ← Execute in parallel after L1
                                ↓
Level 3:                       [H]             ← Execute after L2
                                ↓
...                            ...             ← Continues for N levels
                                ↓
Level N:                       [Z]             ← Final level
```

**Algorithm:**
1. Build dependency graph from control `depends_on` declarations
2. Detect cycles (fail fast with clear error)
3. Handle filtered dependencies (see below)
4. Compute levels via topological sort (supports arbitrary depth)
5. For each level 0..N:
   - Execute all controls at that level concurrently (worker pool)
   - Wait for all to complete
   - Propagate pass/fail status to dependents
6. Skip controls whose dependencies failed (if `skip_on_dependency_failure: true`)

**Filtered dependency handling:**

When a control depends on another control that's excluded by `--tags`, `--severity`, or `--control` filters, the engine must decide what to do. Reglet uses the **skip with warning** approach:

| Scenario | Behavior |
|----------|----------|
| Dependency excluded by filter | Skip dependent control, emit warning |
| Dependency doesn't exist in profile | Error at validation time |
| Dependency failed at runtime | Controlled by `skip_on_dependency_failure` |

**Example:**
```bash
$ reglet check profile.yaml --tags security

⚠ Control 'app-db-queries' skipped: dependency 'db-healthy' excluded by filter
⚠ Control 'end-to-end-flow' skipped: dependency 'app-db-queries' excluded by filter

# Only controls matching --tags security are executed
# Controls depending on filtered-out controls are transitively skipped
```

**Rationale for skip (vs error or implicit include):**
- **Error** would make filters unusable with any dependency graph
- **Implicit include** would execute controls the user explicitly filtered out (surprising)
- **Skip with warning** respects the filter while informing the user of consequences

**Override behavior:**
```bash
# Include dependencies even if they don't match filter
reglet check profile.yaml --tags security --include-dependencies

# This implicitly includes db-healthy (untagged) because app-db-queries needs it
```

**Example with deep dependencies:**
```yaml
controls:
  items:
    # Level 0 - infrastructure checks
    - id: network-reachable
    - id: dns-resolving
    - id: disk-space-ok

    # Level 1 - service connectivity
    - id: db-connectable
      depends_on: [network-reachable]
    - id: cache-connectable
      depends_on: [network-reachable]
    - id: api-dns-valid
      depends_on: [dns-resolving]

    # Level 2 - service health
    - id: db-healthy
      depends_on: [db-connectable]
    - id: cache-healthy
      depends_on: [cache-connectable]
    - id: api-responding
      depends_on: [api-dns-valid, network-reachable]

    # Level 3 - application checks
    - id: app-db-queries
      depends_on: [db-healthy, cache-healthy]
    - id: app-api-integration
      depends_on: [api-responding]

    # Level 4 - integration checks
    - id: end-to-end-flow
      depends_on: [app-db-queries, app-api-integration]

    # Level 5 - final validation
    - id: production-ready
      depends_on: [end-to-end-flow, disk-space-ok]
      skip_on_dependency_failure: true
```

**Execution timeline (6 levels):**
```
Time →
├─ L0: [network] [dns] [disk]                      (3 parallel)
├─ L1: [db-conn] [cache-conn] [api-dns]            (3 parallel)
├─ L2: [db-health] [cache-health] [api-respond]    (3 parallel)
├─ L3: [app-db] [app-api]                          (2 parallel)
├─ L4: [e2e-flow]                                  (1)
├─ L5: [prod-ready]                                (1)
└─ Done
```

**Parallelism controls:**
```bash
# Limit concurrent controls per level (default: NumCPU)
reglet check profile.yaml --parallelism 4

# Sequential execution (disable parallelism)
reglet check profile.yaml --parallelism 1
```

**Within-control parallelism:**
Observations within a single control also run in parallel by default, unless they declare internal dependencies (future: `observation.depends_on`).

### Retry & Flakiness Handling

Infrastructure checks are prone to transient failures (network glitches, rate limits, temporary unavailability). The engine supports automatic retries with configurable backoff.

**Configuration:**
```yaml
controls:
  defaults:
    retries: 3                    # Retry up to 3 times on failure
    retry_delay: 1s               # Initial delay
    retry_backoff: exponential    # none | linear | exponential
    retry_max_delay: 30s          # Cap for exponential backoff

  items:
    - id: api-health
      # Override defaults for this control
      retries: 5                  # More retries for flaky external API
      retry_delay: 2s
      observations:
        - plugin: http
          config:
            url: https://api.example.com/health

    - id: file-check
      retries: 0                  # Disable retries (file checks shouldn't be flaky)
      observations:
        - plugin: file
          config:
            path: /etc/config.yaml
```

**Backoff strategies:**

| Strategy | Behavior | Example (1s initial) |
|----------|----------|----------------------|
| `none` | Fixed delay | 1s, 1s, 1s |
| `linear` | Additive increase | 1s, 2s, 3s |
| `exponential` | Multiplicative increase | 1s, 2s, 4s, 8s... (capped by max_delay) |

**What triggers a retry:**
- Connection timeout
- Connection refused
- DNS resolution failure
- HTTP 5xx responses
- HTTP 429 (rate limited)
- Plugin-reported transient errors

**What does NOT trigger a retry:**
- Expectation failures (check ran, but value was wrong)
- HTTP 4xx responses (except 429)
- Configuration errors
- Plugin-reported permanent errors

**Output shows retry attempts:**
```bash
reglet check profile.yaml --verbose
# ⟳ api-health: attempt 1/3 failed (connection timeout), retrying in 1s...
# ⟳ api-health: attempt 2/3 failed (connection timeout), retrying in 2s...
# ✓ api-health: PASSED (attempt 3/3)
```

**Evidence includes retry info:**
```json
{
  "control": "api-health",
  "status": "passed",
  "attempts": 3,
  "retry_history": [
    {"attempt": 1, "error": "connection timeout", "duration_ms": 5000},
    {"attempt": 2, "error": "connection timeout", "duration_ms": 5000},
    {"attempt": 3, "success": true, "duration_ms": 234}
  ]
}
```

### Rate Limiting

Network plugins making external API calls can overwhelm targets or trigger rate limits. The engine supports configurable rate limiting to prevent this.

**Global rate limit (system config):**

```yaml
# ~/.reglet/config.yaml
rate_limit:
  default: 10/s                    # Default for all plugins
  burst: 20                        # Allow burst up to 20 requests
```

**Per-plugin rate limit:**

```yaml
# ~/.reglet/config.yaml
plugins:
  reglet/http@1.0:
    capabilities:
      - network:outbound:80,443
    rate_limit: 5/s                # Slower for HTTP checks

  reglet/aws@1.0:
    capabilities:
      - network:outbound
    rate_limit: 20/s               # AWS APIs handle more load
```

**Per-observation rate limit:**

```yaml
# profile.yaml
controls:
  items:
    - id: external-api-check
      observations:
        - plugin: http
          config:
            url: https://slow-api.example.com/status
          rate_limit: 1/s          # Very slow external API
```

**Rate limit precedence:**
1. Observation-level `rate_limit` (if specified)
2. Plugin-level `rate_limit` (if specified)
3. Global `rate_limit.default`
4. No limit (if nothing configured)

**Automatic 429 handling:**

When an API returns HTTP 429 (Too Many Requests):
1. Engine respects `Retry-After` header if present
2. Otherwise, uses exponential backoff
3. Counts against retry limit
4. Rate limit automatically reduced for that plugin

```bash
$ reglet check profile.yaml --verbose
# [rate] aws@1.0: HTTP 429 received, backing off 30s
# [rate] aws@1.0: Reducing rate limit from 20/s to 10/s
# [rate] aws@1.0: Retrying in 30s (attempt 2/3)
```

### Error Handling & Recovery

The engine is designed to be resilient. Individual failures are isolated and don't cascade to unrelated controls.

**Per-observation timeout:**

In addition to the global `--timeout` flag, observations can specify individual timeouts:

```yaml
controls:
  items:
    - id: quick-checks
      observations:
        - plugin: file
          config:
            path: /etc/config.yaml
          timeout: 5s                    # Fast timeout for local file

        - plugin: http
          config:
            url: https://slow-api.example.com/health
          timeout: 30s                   # Longer timeout for slow external API
```

**Timeout precedence:**
1. Observation-level `timeout` (if specified)
2. Control-level `timeout` (if specified)
3. Global `--timeout` flag (default: 2m per observation)

**Plugin crash recovery:**

If a plugin crashes or panics mid-execution:

| Scenario | Behavior |
|----------|----------|
| Plugin panics | WASM sandbox catches panic, observation marked as `error` |
| Plugin hangs | Timeout kills execution, observation marked as `error` |
| Plugin returns malformed data | Validation fails, observation marked as `error` |
| Plugin exhausts memory | WASM sandbox terminates, observation marked as `error` |

**Isolation guarantees:**
- Each plugin runs in its own WASM instance
- Plugin crash does NOT affect other observations
- Engine continues executing remaining controls
- Final report includes all results (passed, failed, AND errors)

**Error output:**
```bash
$ reglet check profile.yaml

✓ ssh-config: PASSED
✓ firewall-enabled: PASSED
✗ api-health: FAILED (status_code was 503)
⚠ custom-check: ERROR (plugin crashed: out of memory)
✓ disk-space: PASSED

Summary: 3 passed, 1 failed, 1 error
```

**Capability enforcement failures:**

When a plugin attempts an unauthorized operation:

```bash
$ reglet check profile.yaml --verbose
# [capability] Plugin 'custom-check' attempted fs:read:/etc/shadow
# [capability] DENIED - not in granted capabilities
# [capability] Granted: fs:read:/etc/ssh/**
# ⚠ custom-check: ERROR (capability violation: fs:read:/etc/shadow)
```

**Capability failure behavior:**
- Immediate termination of that observation (no retry)
- Clear error message showing what was attempted vs granted
- Other observations continue unaffected
- Exit code 2 (configuration/execution error)

**Error vs Failure distinction:**

| Status | Meaning | Exit Code |
|--------|---------|-----------|
| `passed` | All expectations met | 0 |
| `failed` | Expectations not met (check ran successfully) | 1 |
| `error` | Check could not run (crash, timeout, capability) | 2 |
| `skipped` | Dependency failed or waiver applied | 0 |

### Iterative Observations (Loops)

Observations can iterate over lists using the `loop` directive. This creates N child observations, one per item.

**Basic loop syntax:**

```yaml
controls:
  items:
    - id: services-reachable
      name: All Services Reachable
      observations:
        - loop:
            items: "{{ .vars.services }}"    # List to iterate over
            as: svc                           # Optional: variable name (default: .loop.item)
          plugin: tcp
          config:
            host: "{{ .loop.item.host }}"    # Access current item
            port: "{{ .loop.item.port }}"
          expect:
            - connected == true
```

**With custom variable name:**

```yaml
- loop:
    items: "{{ .vars.databases }}"
    as: db                                   # Custom name
  plugin: tcp
  config:
    host: "{{ .db.host }}"                   # Use custom name
    port: "{{ .db.port }}"
```

**Loop execution model:**

```
Observation with loop:
├─ items: [db-1, db-2, db-3, api-1, api-2]
│
├─ Child Observation [0]: db-1
│   ├─ Execute plugin
│   ├─ Evaluate expect
│   └─ Result: PASSED ✓
│
├─ Child Observation [1]: db-2
│   ├─ Execute plugin
│   ├─ Evaluate expect
│   └─ Result: PASSED ✓
│
├─ Child Observation [2]: db-3
│   ├─ Execute plugin
│   ├─ Evaluate expect
│   └─ Result: FAILED ✗    ← One failure
│
├─ Child Observation [3]: api-1
│   └─ Result: PASSED ✓
│
├─ Child Observation [4]: api-2
│   └─ Result: PASSED ✓
│
└─ Aggregated Result: FAILED (1/5 failed)
```

**Aggregation rules:**

| Scenario | Control Status |
|----------|----------------|
| All children pass | `passed` |
| Any child fails | `failed` |
| All children skipped | `skipped` |
| Mix of pass/skip | `passed` |

**Aggregation strategies:**

```yaml
- id: services-reachable
  observations:
    - loop:
        items: "{{ .vars.services }}"
        # Aggregation strategy (default: all)
        aggregate: all        # ALL must pass (default)
        # aggregate: any      # At least ONE must pass
        # aggregate: majority # >50% must pass
        # aggregate: count:3  # At least 3 must pass
      plugin: tcp
      config:
        host: "{{ .loop.item.host }}"
```

**Evidence collection:**

Each loop iteration produces its own evidence. The parent observation collects all child evidence:

```yaml
evidence:
  observation: services-reachable
  loop:
    total: 5
    passed: 4
    failed: 1
    aggregate: all
    status: failed           # Because aggregate=all and 1 failed
  children:
    - index: 0
      item: {host: "db-1.example.com", port: 5432}
      status: passed
      evidence:
        connected: true
        latency_ms: 12

    - index: 1
      item: {host: "db-2.example.com", port: 5432}
      status: passed
      evidence:
        connected: true
        latency_ms: 8

    - index: 2
      item: {host: "db-3.example.com", port: 5432}
      status: failed
      evidence:
        connected: false
        error: "connection refused"

    - index: 3
      item: {host: "api-1.example.com", port: 443}
      status: passed
      evidence:
        connected: true

    - index: 4
      item: {host: "api-2.example.com", port: 443}
      status: passed
      evidence:
        connected: true
```

**Parallel loop execution:**

Loop iterations run in parallel by default (respecting `--parallelism`):

```yaml
- loop:
    items: "{{ .vars.services }}"
    parallel: true           # Default: true
    # parallel: false        # Sequential execution
    # parallel: 3            # Max 3 concurrent
  plugin: http
  config:
    url: "https://{{ .loop.item.host }}/health"
```

**Loop with index:**

```yaml
- loop:
    items: "{{ .vars.servers }}"
  plugin: command
  config:
    run: "echo 'Checking server {{ .loop.index }}: {{ .loop.item.name }}'"
  # .loop.index = 0, 1, 2, ...
  # .loop.first = true/false
  # .loop.last = true/false
  # .loop.length = total items
```

**Complex iteration patterns:**

For scenarios requiring nested iteration (e.g., cross-product of regions), pre-compute the combinations in the `vars` section rather than using nested loops. This keeps the observation logic simple and explicit.

```yaml
# Instead of nested loops, pre-compute combinations
vars:
  regions: [us-east-1, us-west-2, eu-west-1]

  # Pre-compute cross-region pairs (excluding self)
  region_pairs:
    - { src: us-east-1, dst: us-west-2 }
    - { src: us-east-1, dst: eu-west-1 }
    - { src: us-west-2, dst: us-east-1 }
    - { src: us-west-2, dst: eu-west-1 }
    - { src: eu-west-1, dst: us-east-1 }
    - { src: eu-west-1, dst: us-west-2 }

controls:
  items:
    - id: cross-region-connectivity
      observations:
        - loop:
            items: "{{ .vars.region_pairs }}"
          plugin: tcp
          config:
            from_region: "{{ .loop.item.src }}"
            to_host: "gateway.{{ .loop.item.dst }}.example.com"
          expect:
            - connected == true
```

**Why this approach:**
- Explicit: all test cases visible in vars (easier to review)
- Debuggable: can add/remove pairs individually
- Flexible: supports any filtering logic (not just `exclude_self`)
- Simple: no nested loop syntax to learn

**For dynamic generation**, use `reglet import` or external tooling to generate the vars section, or use a profile template (Phase 3).

**CLI output for loops:**

```
$ reglet check profile.yaml

✗ services-reachable (4/5 passed)
  ✓ db-1.example.com:5432 - connected
  ✓ db-2.example.com:5432 - connected
  ✗ db-3.example.com:5432 - connection refused
  ✓ api-1.example.com:443 - connected
  ✓ api-2.example.com:443 - connected
```

### Expect Expression Language

Expectations use the [expr-lang](https://expr-lang.org/) expression language:

**Operators:**

| Category | Operators |
|----------|-----------|
| Comparison | `==`, `!=`, `<`, `>`, `<=`, `>=` |
| Logical | `&&`, `\|\|`, `!`, `and`, `or`, `not` |
| String | `contains`, `startsWith`, `endsWith`, `matches` |
| Collection | `in`, `not in`, `all`, `any`, `one`, `none` |
| Null | `??` (nil coalescing) |

**Examples:**
```yaml
expect:
  # Simple comparisons
  - status_code == 200
  - response_time_ms < 500

  # String operations
  - body contains "healthy"
  - content matches "PermitRootLogin\\s+no"

  # Logical combinations
  - status_code == 200 && response_time_ms < 1000

  # Collection operations
  - all(ports, {# in [80, 443]})

  # Null handling
  - (error ?? "") == ""
```

### Evidence Collection

Every observation captures evidence:

```yaml
evidence:
  timestamp: "2025-01-15T10:30:00Z"
  plugin: file
  config:
    path: /etc/ssh/sshd_config
  data:
    exists: true
    permissions: "0600"
    owner: root
    group: root
    size: 3892
    content_hash: "sha256:abc123..."
  raw: |
    # Full file content (if configured to capture)
    PermitRootLogin no
    ...
```

**Evidence size limits:**

Raw evidence is truncated to prevent output explosion and memory exhaustion:

| Evidence Type | Default Limit | Configurable |
|---------------|---------------|--------------|
| `raw` content (file, command output) | 1 MB | Yes |
| `response.body` (HTTP) | 1 MB | Yes |
| Total evidence per observation | 5 MB | Yes |
| Binary content | Not captured | N/A |

```yaml
# Override defaults in system config
evidence:
  limits:
    raw_content: 2MB        # Increase for large configs
    http_body: 512KB        # Decrease for API checks
    total_per_observation: 10MB
```

**Truncation behavior:**
```yaml
evidence:
  raw: |
    # First 1MB of content...
    [TRUNCATED: 99MB omitted, full size: 100MB]
  truncated: true
  original_size: 104857600
  content_hash: "sha256:abc123..."  # Hash computed on FULL content
```

> **Note:** The `content_hash` is always computed on the full, untruncated content. This allows auditors to verify integrity even when raw content is truncated.

**Binary file handling:**
```yaml
# Binary files don't capture raw content
evidence:
  data:
    exists: true
    size: 52428800
    is_binary: true
    content_hash: "sha256:def456..."
  raw: "[BINARY: 50MB, hash: sha256:def456...]"
```

### Evidence Redaction

⚠️ **Security**: Evidence can contain secrets. Raw file contents, API responses, and command output may include credentials, tokens, or PII. Reglet automatically redacts sensitive data before writing to output.

**The problem:**

```yaml
# Profile checks a config file
- plugin: file
  config:
    path: /etc/myapp/config.yaml

# Evidence captures raw content - LEAKED SECRETS!
evidence:
  raw: |
    database:
      host: db.example.com
      password: "super-secret-password"  # 😱 Now in your audit report
    api_key: "sk-live-abc123..."         # 😱 Sent to Enterprise Vault
```

**Solution: Multi-layer redaction**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Evidence Pipeline                             │
├─────────────────────────────────────────────────────────────────┤
│  1. Plugin marks fields as sensitive                            │
│     └─▶ Plugin knows "password" field is secret                 │
│                                                                  │
│  2. Engine applies pattern-based redaction                      │
│     └─▶ Regex for API keys, tokens, passwords                   │
│                                                                  │
│  3. User-defined redaction rules                                │
│     └─▶ Custom patterns for org-specific secrets                │
│                                                                  │
│  4. Output receives sanitized evidence                          │
│     └─▶ Secrets replaced with [REDACTED] or SHA256 hash         │
└─────────────────────────────────────────────────────────────────┘
```

**1. Observation-level `sensitive` flag:**

```yaml
controls:
  items:
    - id: db-config-check
      observations:
        - plugin: file
          config:
            path: /etc/myapp/config.yaml
          sensitive: true              # ← Redact ALL raw content
          expect:
            - exists == true
            - content contains "ssl: true"

        - plugin: http
          config:
            url: https://api.example.com/config
            headers:
              Authorization: "{{ .secrets.api_token }}"
          sensitive:                   # ← Selective redaction
            - response.body            # Redact response body
            - request.headers          # Redact request headers (contains auth)
          expect:
            - status_code == 200
```

**2. Plugin-declared sensitive fields:**

Plugins declare which output fields contain sensitive data:

```wit
// In plugin WIT interface
record evidence {
    timestamp: string,
    data: list<tuple<string, value>>,
    raw: option<string>,
    sensitive-fields: list<string>,    // ← Plugin declares sensitive fields
}
```

```yaml
# Plugin: http automatically marks these as sensitive
sensitive-fields:
  - request.headers.Authorization
  - request.headers.Cookie
  - response.headers.Set-Cookie
```

**3. Built-in redaction patterns:**

The engine automatically detects and redacts common secret patterns:

| Pattern | Example | Redacted To |
|---------|---------|-------------|
| AWS Access Key | `AKIA...` | `[AWS_KEY:sha256:abc1...]` |
| AWS Secret Key | `wJalrXUtn...` | `[REDACTED]` |
| GitHub Token | `ghp_...`, `gho_...` | `[GITHUB_TOKEN:sha256:...]` |
| Generic API Key | `api_key=...` | `[API_KEY:sha256:...]` |
| Private Key | `-----BEGIN RSA PRIVATE KEY-----` | `[PRIVATE_KEY:REDACTED]` |
| Password fields | `password: ...` | `[PASSWORD:REDACTED]` |
| JWT | `eyJ...` (3 parts) | `[JWT:sha256:...]` |
| Connection strings | `postgres://user:pass@...` | `postgres://user:[REDACTED]@...` |

**4. User-defined redaction rules:**

```yaml
# ~/.reglet/config.yaml
redaction:
  # Additional patterns to redact
  patterns:
    - name: internal-api-key
      regex: 'INTERNAL-[A-Z0-9]{32}'
      replace: '[INTERNAL_KEY:REDACTED]'

    - name: employee-id
      regex: 'EMP-\d{6}'
      replace: '[EMPLOYEE_ID]'

  # Always redact these JSON paths
  paths:
    - '**.password'
    - '**.secret'
    - '**.credentials.*'
    - '**.privateKey'

  # Hash instead of full redaction (proves uniqueness)
  hash_mode:
    enabled: true
    algorithm: sha256
    truncate: 8        # First 8 chars of hash
```

**Regex safety (ReDoS protection):**

User-defined regex patterns are validated and sandboxed to prevent ReDoS attacks:

| Protection | Description |
|------------|-------------|
| Pattern timeout | Each regex match times out after 100ms |
| Complexity limit | Patterns with excessive backtracking rejected at config load |
| Input chunking | Large inputs processed in 64KB chunks |
| Built-in patterns | Pre-validated, cannot cause ReDoS |

```yaml
# This pattern would be REJECTED at config load time:
redaction:
  patterns:
    - name: bad-pattern
      regex: '(a+)+$'    # ❌ Rejected: catastrophic backtracking

# Error:
# redaction.patterns[0].regex: pattern rejected - potential ReDoS
# Hint: avoid nested quantifiers like (a+)+ or (a|a)+
```

**Safe pattern guidelines:**
- Avoid nested quantifiers: `(a+)+`, `(a*)*`, `(a|a)+`
- Use possessive quantifiers where supported: `a++` instead of `a+`
- Prefer character classes over alternation: `[ab]` instead of `(a|b)`
- Anchor patterns when possible: `^pattern$`

**5. Redaction modes:**

```bash
# Default: redact secrets, keep hashes for correlation
reglet check profile.yaml

# Full redaction: no hashes, just [REDACTED]
reglet check profile.yaml --redact-full

# No redaction (DANGEROUS - dev/debug only)
reglet check profile.yaml --no-redact
# ⚠️  WARNING: Output may contain secrets. Do not share or store.

# Show what would be redacted
reglet check profile.yaml --redact-preview
```

**Redacted evidence output:**

```yaml
evidence:
  timestamp: "2025-01-15T10:30:00Z"
  plugin: file
  config:
    path: /etc/myapp/config.yaml
  data:
    exists: true
    permissions: "0600"
    content_hash: "sha256:abc123..."
  raw: |
    database:
      host: db.example.com
      password: "[REDACTED]"
    api_key: "[API_KEY:sha256:7f3d...]"
  redaction_applied:
    - field: raw
      patterns_matched: ["password", "api_key"]
      redacted_at: "2025-01-15T10:30:00Z"
```

**Evidence for auditors:**

Redaction doesn't weaken compliance evidence - auditors need to know:
- ✅ The file exists with correct permissions
- ✅ The configuration structure is correct
- ✅ Required settings are present
- ❌ They do NOT need to see actual password values

```yaml
# Auditor-friendly evidence (sensitive: true on observation)
evidence:
  plugin: file
  data:
    exists: true
    permissions: "0600"          # ✓ Proves file is protected
    owner: root                  # ✓ Proves correct ownership
    content_hash: "sha256:..."   # ✓ Proves file hasn't changed
    contains_required_settings:  # ✓ Proves compliance
      - ssl_enabled: true
      - password_present: true   # Field exists, value redacted
```

### Control Dependencies

Controls can declare dependencies on other controls:

```yaml
controls:
  items:
    - id: network-connectivity
      observations:
        - plugin: tcp
          config:
            host: db.example.com
            port: 5432

    - id: database-queries
      depends_on:
        - network-connectivity
      skip_on_dependency_failure: true
      observations:
        - plugin: command
          config:
            run: psql -c "SELECT 1"
```

### Waivers & Suppressions (OSS)

Controls may fail for known, accepted reasons (legacy systems, compensating controls, business exceptions). The OSS version provides basic suppression via `.regletignore`. Enterprise adds approval workflows, expiration, and audit trails.

**`.regletignore` file:**

```yaml
# .regletignore (committed to repo, documents accepted risks)
waivers:
  # Simple: suppress control everywhere
  - control: legacy-port-check
    reason: "Legacy system, decommissioning in Q2"

  # Scoped: suppress only for specific targets/contexts
  - control: ssh-root-disabled
    scope:
      targets: [legacy-server-1, legacy-server-2]
    reason: "Legacy systems require root access for vendor support"
    ticket: "SEC-1234"

  # Observation-level: suppress specific observation within a control
  - control: api-health
    observation: 1                    # Index of observation in control
    reason: "Staging API has known latency issues"
    scope:
      contexts: [staging]

  # Temporary: include expiration (OSS shows warning, Enterprise enforces)
  - control: cert-expiry-check
    reason: "Cert renewal in progress"
    ticket: "OPS-5678"
    expires: 2025-02-01               # OSS: warning after expiry, Enterprise: hard fail
```

**Behavior:**

```bash
# Run with waivers applied
reglet check profile.yaml
# ⚠ ssh-root-disabled: WAIVED (legacy-server-1) - "Legacy systems require root..."
# ✓ ssh-root-disabled: PASSED (web-server-1)
# ✓ All other controls...
#
# Summary: 15 passed, 2 waived, 0 failed

# Waived controls don't fail the exit code (by default)
echo $?  # 0

# Strict mode: waivers still count as findings
reglet check profile.yaml --strict-waivers
echo $?  # 1 (because waivers exist)

# Show all waivers
reglet waivers list
# CONTROL              SCOPE                REASON                         EXPIRES
# ssh-root-disabled    legacy-server-*      Legacy vendor support          -
# cert-expiry-check    *                    Cert renewal in progress       2025-02-01

# Expired waiver warning
reglet check profile.yaml
# ⚠ WARNING: Waiver for 'cert-expiry-check' expired on 2025-02-01
#   Consider removing from .regletignore or extending
```

**Output includes waiver status:**

```json
{
  "findings": [
    {
      "control": "ssh-root-disabled",
      "status": "waived",
      "waiver": {
        "reason": "Legacy systems require root access for vendor support",
        "ticket": "SEC-1234",
        "scope": ["legacy-server-1", "legacy-server-2"]
      }
    }
  ]
}
```

**OSS vs Enterprise waivers:**

| Feature | OSS | Enterprise |
|---------|-----|------------|
| Suppress by control ID | ✓ | ✓ |
| Scope to targets/contexts | ✓ | ✓ |
| Reason documentation | ✓ | ✓ |
| Ticket reference | ✓ | ✓ |
| Expiration dates | Warning only | Hard enforcement |
| Approval workflow | - | ✓ (multi-person approval) |
| Audit trail | Git history | Full audit log |
| Centralized management | - | ✓ (dashboard) |
| Auto-expire notifications | - | ✓ (Slack/email alerts) |

---

## 7. OSCAL Integration

### Supported OSCAL Artifacts

**Assessment Results**: Output of validation runs
```bash
reglet check profile.yaml --output oscal-ar > assessment-results.json
```

**Plan of Action & Milestones (POA&M)**: Remediation tracking for failed controls
```bash
reglet poam profile.yaml --output oscal > poam.json
```

### OSCAL Assessment Results Structure

```json
{
  "assessment-results": {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "metadata": {
      "title": "Reglet Assessment Results",
      "last-modified": "2025-01-15T10:30:00Z",
      "version": "1.0.0"
    },
    "import-ap": {
      "href": "#profile"
    },
    "results": [
      {
        "uuid": "...",
        "title": "Production Baseline Assessment",
        "start": "2025-01-15T10:30:00Z",
        "end": "2025-01-15T10:30:45Z",
        "observations": [
          {
            "uuid": "...",
            "title": "SSH Root Login Check",
            "methods": ["AUTOMATED"],
            "subjects": [
              {
                "type": "component",
                "subject-uuid": "..."
              }
            ],
            "collected": "2025-01-15T10:30:05Z"
          }
        ],
        "findings": [
          {
            "uuid": "...",
            "title": "SSH Root Login Disabled",
            "target": {
              "type": "objective-id",
              "target-id": "ssh-root-disabled",
              "status": {
                "state": "satisfied"
              }
            }
          }
        ]
      }
    ]
  }
}
```

**Retry history mapping:**

When observations are retried (due to transient failures), the `retry_history` from Reglet's internal format maps to OSCAL as follows:

| Reglet Field | OSCAL Location | Notes |
|--------------|----------------|-------|
| `retry_history[]` | `observations[].props[]` | Each attempt stored as a property |
| `retry_history[].attempt` | `prop.name="attempt-N"` | N = attempt number |
| `retry_history[].error` | `prop.value` | Error message for failed attempts |
| `retry_history[].duration_ms` | `prop.remarks` | Timing information |
| Final success | `observations[].collected` | Only successful attempt timestamp |

Example OSCAL observation with retry history:
```json
{
  "uuid": "...",
  "title": "API Health Check",
  "methods": ["AUTOMATED"],
  "collected": "2025-01-15T10:30:15Z",
  "props": [
    {"name": "attempt-1", "value": "connection timeout", "remarks": "5000ms"},
    {"name": "attempt-2", "value": "connection timeout", "remarks": "5000ms"},
    {"name": "attempt-3", "value": "success", "remarks": "234ms"}
  ]
}
```

### Output Attestation (Proof of Origin)

Raw JSON/OSCAL output proves nothing - anyone can create a file claiming compliance. Reglet supports cryptographic signing to prove the output was generated by a real scan.

**Local key signing (OSS):**
```bash
# Generate a signing key (one-time setup)
reglet keys generate
# Created ~/.reglet/keys/signing.key (private)
# Created ~/.reglet/keys/signing.pub (public - share with auditors)

# Sign output during scan
reglet check profile.yaml --output json --sign > results.json

# Creates two files:
# - results.json (the assessment results)
# - results.json.sig (detached signature)

# Or inline signature (single file)
reglet check profile.yaml --output json --sign --sign-inline > results.signed.json
```

**Signed output structure (inline):**
```json
{
  "attestation": {
    "version": "1.0",
    "signed_at": "2025-01-15T10:30:45Z",
    "signer": {
      "key_id": "sha256:abc123...",
      "public_key": "-----BEGIN PUBLIC KEY-----\n..."
    },
    "signature": "MEUCIQD...",
    "algorithm": "ECDSA-P256-SHA256"
  },
  "assessment_results": {
    // ... actual results ...
  }
}
```

**Verification:**
```bash
# Verify signed output
reglet verify results.json --pubkey auditor-key.pub
# ✓ Signature valid
# ✓ Signed by: sha256:abc123...
# ✓ Signed at: 2025-01-15T10:30:45Z
# ✓ Results have not been tampered with

# Verify with detached signature
reglet verify results.json --signature results.json.sig --pubkey auditor-key.pub
```

**Keyless signing with Sigstore (optional):**
```bash
# Sign using OIDC identity (no key management)
reglet check profile.yaml --output json --sign-keyless > results.json

# Uses Sigstore/Cosign for keyless signing
# Signer identity tied to your OIDC provider (GitHub, Google, etc.)
# Recorded in public transparency log (Rekor)
```

**What attestation includes:**
- Cryptographic signature over results
- Timestamp of scan
- Signer identity (key fingerprint or OIDC identity)
- Reglet version used
- Profile hash (proves which profile was used)
- Plugin versions and checksums (proves which plugins ran)

**Verification for auditors:**
```bash
# Full verification report
reglet verify results.json --pubkey company-audit-key.pub --verbose

# Attestation Verification Report
# ================================
# Signature:       VALID ✓
# Signer:          sha256:abc123... (company-audit-key.pub)
# Signed at:       2025-01-15T10:30:45Z
# Reglet:      v1.2.3
# Profile:         production-baseline v1.0.0 (sha256:def456...)
# Plugins:
#   - reglet/http@1.0.2 (sha256:789...)
#   - reglet/file@1.0.0 (sha256:012...)
# Results:         NOT TAMPERED ✓
```

**OSS vs Enterprise attestation:**

| Feature | OSS | Enterprise |
|---------|-----|------------|
| Local key signing | ✓ | ✓ |
| Sigstore keyless signing | ✓ | ✓ |
| Timestamping authority (RFC 3161) | - | ✓ |
| Hardware security module (HSM) | - | ✓ |
| Centralized key management | - | ✓ |
| Audit log of all signed reports | - | ✓ |

### JUnit XML Output

For CI systems that parse JUnit XML (Jenkins, Azure DevOps, GitLab, CircleCI), Reglet provides native JUnit output:

```bash
reglet check profile.yaml --output junit > results.xml
```

**Mapping to JUnit:**

| Reglet Concept | JUnit Element | Notes |
|----------------|---------------|-------|
| Profile | `<testsuite>` | One suite per profile |
| Control | `<testcase>` | One testcase per control |
| Pass | (no child element) | Normal JUnit pass |
| Fail | `<failure>` | Includes expect expression that failed |
| Error | `<error>` | Plugin crash, timeout, capability denied |
| Skip | `<skipped>` | Filtered out or dependency missing |
| Duration | `time` attribute | Seconds with millisecond precision |
| Control ID | `name` attribute | |
| Plugin | `classname` attribute | e.g., `reglet.file` |

**Example JUnit output:**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="production-baseline" tests="5" failures="1" errors="0" skipped="1" time="2.345">
    <testcase name="ssh-root-disabled" classname="reglet.file" time="0.234"/>
    <testcase name="api-health" classname="reglet.http" time="1.456">
      <failure message="expect: status_code == 200" type="AssertionError">
        Expected: status_code == 200
        Actual: status_code = 503

        Evidence:
          url: https://api.example.com/health
          response_time_ms: 1456
      </failure>
    </testcase>
    <testcase name="db-backup-exists" classname="reglet.file" time="0.123">
      <skipped message="dependency 'db-connectable' excluded by filter"/>
    </testcase>
  </testsuite>
</testsuites>
```

**CI integration examples:**

```yaml
# Jenkins Pipeline
steps {
  sh 'reglet check profile.yaml --output junit > reglet-results.xml'
  junit 'reglet-results.xml'
}

# Azure DevOps
- script: reglet check profile.yaml --output junit > $(System.DefaultWorkingDirectory)/results.xml
- task: PublishTestResults@2
  inputs:
    testResultsFormat: 'JUnit'
    testResultsFiles: '**/results.xml'

# GitLab CI
compliance:
  script:
    - reglet check profile.yaml --output junit > results.xml
  artifacts:
    reports:
      junit: results.xml
```

---

## 8. CLI Interface

### Command Structure

```
reglet <command> [subcommand] [flags] [arguments]
```

### Commands

**check** - Execute profile validation
```bash
reglet check profile.yaml
reglet check profile.yaml --context production
reglet check profile.yaml --output json
reglet check profile.yaml --fail-on critical,high
reglet check --ci  # CI mode: minimal output, strict exit codes

# Save results for later comparison
reglet check profile.yaml -o json > results-2025-01-15.json

# Compare against previous run (regression detection)
reglet check profile.yaml --diff results-2025-01-14.json
# Output shows: new failures, new passes, unchanged
```

**plan** - Dry-run to see what would execute (no actual checks)
```bash
# Show execution plan without running anything
reglet plan profile.yaml

# Output:
# ═══════════════════════════════════════════════════════════
# Execution Plan for: production-baseline v1.0.0
# Context: production
# ═══════════════════════════════════════════════════════════
#
# Plugins to load:
#   ✓ reglet/http@1.0.2 (cached)
#   ✓ reglet/file@1.0.0 (cached)
#   ↓ reglet/aws@1.0.1 (will download)
#
# Execution order (4 levels, 12 controls):
#
# Level 0 (parallel):
#   [network-check]
#     → plugin: tcp
#     → config: host=api.example.com, port=443
#     → expect: connected == true
#
#   [dns-check]
#     → plugin: dns
#     → config: name=api.example.com, type=A
#     → expect: resolved == true
#
# Level 1 (depends on L0):
#   [api-health] depends_on: [network-check, dns-check]
#     → plugin: http
#     → config: url=https://api.example.com/health, timeout=5s
#     → expect: status_code == 200, response_time_ms < 500
#     → retries: 3 (exponential backoff, max 30s)
#
# Level 2 (depends on L1):
#   [api-data-check] depends_on: [api-health]
#     → plugin: http
#     → config: url=https://api.example.com/validate
#     → expect: body contains "valid"
#     → skip_on_dependency_failure: true
#
# Waivers applied:
#   - legacy-port-check (global): "Legacy system, decommissioning Q2"
#   - ssh-root-disabled (legacy-server-1): "Vendor support requirement"
#
# Estimated network calls: 8
# Estimated file reads: 3
# Estimated commands: 2
#
# Run 'reglet check profile.yaml' to execute.

# Plan with variable resolution shown
reglet plan profile.yaml --show-resolved
# Shows actual values after template resolution

# Plan for specific context
reglet plan profile.yaml --context staging
# Shows what would run in staging context

# Validate only (syntax + schema, no execution plan)
reglet plan profile.yaml --validate-only
# ✓ Profile syntax valid
# ✓ All plugins available
# ✓ All control IDs unique
# ✓ No circular dependencies
# ✓ All referenced variables defined
```

**Why plan mode matters:**
- Debug complex profiles before running
- See resolved template variables
- Understand execution order/parallelism
- Identify missing plugins before download
- Validate without hitting production systems
- Estimate execution scope (network calls, etc.)

**import** - Generate baseline profile from existing infrastructure

> ⚠️ **IMPORTANT: Import is bootstrap-only (v1)**
>
> Import generates a *new* profile file. It does **not** merge with existing profiles.
> After customizing an imported profile, subsequent imports will NOT preserve your changes.
>
> **Recommended workflow:**
> 1. `reglet import aws` → generates `aws-baseline.yaml`
> 2. Customize: add severity, owners, mappings, waivers
> 3. Periodically run `reglet import aws --diff aws-baseline.yaml` to detect drift
> 4. Manually merge new resources into your customized profile
>
> Merge mode (`--merge`) is planned for Phase 5+.

```bash
# Import from AWS (scans your account, generates profile)
reglet import aws --region us-east-1
# Scanning AWS account...
#   Found 12 EC2 instances
#   Found 5 S3 buckets
#   Found 3 RDS databases
#   Found 8 Security Groups
# Generated: aws-baseline.yaml (47 controls)
#
# Review and customize, then run:
#   reglet check aws-baseline.yaml

# Import from Kubernetes cluster
reglet import k8s --context production
# Scanning cluster...
#   Found 23 Deployments
#   Found 15 Services
#   Found 8 ConfigMaps with sensitive patterns
# Generated: k8s-baseline.yaml (31 controls)

# Import from local system
reglet import local
# Scanning local system...
#   Detected: Ubuntu 22.04
#   Found: sshd, nginx, postgresql services
#   Found: /etc/ssh/sshd_config, /etc/nginx/nginx.conf
# Generated: local-baseline.yaml (18 controls)

# Import from Terraform state
reglet import terraform --state terraform.tfstate
# Parsing Terraform state...
#   Found 34 managed resources
# Generated: terraform-baseline.yaml (22 controls)

# Import from Docker Compose
reglet import compose --file docker-compose.yaml
# Parsing compose file...
#   Found 5 services
# Generated: compose-baseline.yaml (12 controls)

# Import with compliance framework mapping
reglet import aws --region us-east-1 --framework iso27001
# Generates profile with ISO27001 control mappings pre-filled
```

**What import generates:**
```yaml
# aws-baseline.yaml (auto-generated, customize as needed)
profile:
  name: aws-baseline
  version: 1.0.0
  description: Auto-generated from AWS account scan on 2025-01-15
  generated_by: reglet import aws

plugins:
  - reglet/aws@1.0

controls:
  defaults:
    severity: warning
    owner: TODO  # Set your team
    tags: [aws, auto-generated]

  items:
    # --- S3 Buckets ---
    - id: s3-my-app-bucket-public-access
      name: "S3 bucket 'my-app-bucket' blocks public access"
      description: Auto-generated check for S3 bucket public access settings
      # TODO: Review and adjust severity
      observations:
        - plugin: aws
          config:
            service: s3
            resource: my-app-bucket
            check: public_access_blocked
          expect:
            - block_public_acls == true
            - block_public_policy == true
      remediation: |
        Enable S3 Block Public Access in AWS Console or via CLI:
        aws s3api put-public-access-block --bucket my-app-bucket ...

    # --- EC2 Instances ---
    - id: ec2-i-1234567-imdsv2
      name: "EC2 instance 'web-server-1' uses IMDSv2"
      observations:
        - plugin: aws
          config:
            service: ec2
            instance: i-1234567
            check: imdsv2_required
          expect:
            - http_tokens == "required"

    # ... more auto-generated controls ...
```

**Why import matters:**
- **Instant value**: See your infrastructure through Reglet's lens immediately
- **No blank page**: Start with a real profile, then customize
- **Discovery**: Find resources you didn't know about
- **Baseline**: Capture current state before making changes
- **Onboarding**: New team members understand infrastructure quickly

**Import is read-only:**
- Only queries/scans - never modifies infrastructure
- Credentials needed for cloud imports (uses standard SDK auth)
- Local imports use filesystem/process inspection

**⚠️ Import is bootstrap-only (v1):**

Import generates a *new* profile file. It does **not** merge with existing profiles.

```bash
# First import - creates aws-baseline.yaml
reglet import aws --region us-east-1
# Generated: aws-baseline.yaml (47 controls)

# 6 months later, team added 50 new S3 buckets...
# Running import again creates a NEW file, does NOT merge
reglet import aws --region us-east-1
# Generated: aws-baseline.yaml (97 controls)
# ⚠️  WARNING: aws-baseline.yaml already exists
#     Use --output aws-baseline-new.yaml to avoid overwriting
#     Or use --force to overwrite (your customizations will be lost)
```

**The drift problem:**

| Time | Profile State | Reality |
|------|---------------|---------|
| Day 0 | Import: 47 controls | 47 resources |
| Day 30 | 47 controls (customized) | 60 resources |
| Day 180 | 47 controls (customized) | 120 resources |

Your profile is now missing 73 resources. Options:

1. **Re-import + manual merge** (tedious but safe)
   ```bash
   reglet import aws -o aws-new.yaml
   diff aws-baseline.yaml aws-new.yaml  # Manual review
   ```

2. **Diff mode** - see what's missing (OSS)
   ```bash
   reglet import aws --diff aws-baseline.yaml
   # Shows: 73 new resources not in profile
   # Shows: 5 resources in profile no longer exist
   ```

3. **Merge mode** (Future - Phase 5+)
   ```bash
   # Future: intelligent merge preserving customizations
   reglet import aws --merge aws-baseline.yaml
   # ✓ Added 73 new controls
   # ✓ Preserved 47 existing controls (with your customizations)
   # ⚠ 5 controls reference deleted resources (marked for review)
   ```

**Recommended workflow (v1):**

```bash
# Initial setup
reglet import aws -o aws-baseline.yaml
# Customize: add severity, owners, mappings, waivers

# Periodic drift check (doesn't modify your profile)
reglet import aws --diff aws-baseline.yaml

# If drift detected, manually review and update
reglet import aws -o aws-latest.yaml
diff aws-baseline.yaml aws-latest.yaml
# Manually merge new controls into your customized profile
```

**init** - Interactive setup wizard
```bash
reglet init
reglet init --template compliance
reglet init --template infrastructure
reglet init --non-interactive --name my-project --type aws
```

**Interactive wizard flow:**

```
$ reglet init

Welcome to Reglet! Let's set up your compliance profile.

? What type of project is this?
  ❯ AWS Infrastructure
    Kubernetes Cluster
    Linux Server
    Web Application
    Empty Profile

? What is the project name? [my-project]: production-api

? What compliance frameworks do you need? (space to select, enter to confirm)
  ❯ ◉ SOC 2 Type II
    ◉ ISO 27001
    ◯ PCI DSS
    ◯ HIPAA
    ◯ FedRAMP
    ◯ CIS Benchmarks
    ◯ Custom only

? Do you want to enable CI/CD integration? [Y/n]: y

? Select your CI/CD platform:
  ❯ GitHub Actions
    GitLab CI
    Azure DevOps
    CircleCI
    Jenkins
    None (manual runs only)

? Where should compliance checks run?
  ❯ ◉ On pull requests
    ◉ On push to main
    ◯ On schedule (nightly)
    ◯ Manual trigger only

Creating project structure...

✓ Created reglet.yaml (12 controls from SOC2, ISO27001)
✓ Created reglet-system.yaml (plugin capabilities)
✓ Created .github/workflows/compliance.yaml
✓ Created .regletignore (empty)

Next steps:
  1. Review reglet.yaml and customize for your environment
  2. Run 'reglet check' to validate your current state
  3. Run 'reglet import' to auto-discover additional checks
  4. Commit the new files to your repository

$ reglet check
```

**Template presets:**

| Template | Includes | Use Case |
|----------|----------|----------|
| `aws` | IAM, S3, EC2, VPC controls | AWS infrastructure |
| `kubernetes` | Pod security, RBAC, network policies | K8s clusters |
| `linux` | SSH, firewall, users, services | Linux servers |
| `webapp` | TLS, headers, CORS, CSP | Web applications |
| `compliance` | Framework-specific controls | Audit preparation |
| `empty` | Minimal skeleton | Custom from scratch |

```bash
# Skip wizard, use preset
reglet init --template aws --name my-aws-project --non-interactive

# Skip wizard, specify frameworks
reglet init --template compliance --frameworks soc2,iso27001 --ci github
```

**explain** - Get information about controls and plugins
```bash
reglet explain ssh-root-disabled
reglet explain --plugin http
reglet explain --catalog iso27001
```

**fix** - Auto-remediate issues (where supported)

⚠️ **Safety**: Fix is interactive by default. Each fix requires confirmation before applying.

```bash
# Default: interactive mode - prompts for each fix
reglet fix profile.yaml

# Dry-run: show what would be fixed without making changes
reglet fix profile.yaml --dry-run

# Fix specific control only
reglet fix profile.yaml --control ssh-root-disabled

# Non-interactive (CI/CD only - use with caution)
reglet fix profile.yaml --yes
```

**Interactive mode (default):**

```
$ reglet fix profile.yaml

Analyzing 3 failed controls for available fixes...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[1/3] ssh-root-disabled (CRITICAL)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Status: FAILED - PermitRootLogin is set to 'yes'

Proposed fix:
  1. sed -i 's/PermitRootLogin yes/PermitRootLogin no/g' /etc/ssh/sshd_config
  2. systemctl reload sshd

Apply this fix? [y/N/d/s/?]
  y = apply fix
  N = skip (default)
  d = show diff preview
  s = skip all remaining
  ? = explain fix in detail

> d

--- /etc/ssh/sshd_config (current)
+++ /etc/ssh/sshd_config (after fix)
@@ -12,7 +12,7 @@
 # Authentication:
-PermitRootLogin yes
+PermitRootLogin no

Apply this fix? [y/N/d/s/?] y

✓ Fix applied successfully
✓ sshd reloaded

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[2/3] firewall-enabled (HIGH)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Status: FAILED - firewalld service is not running

Proposed fix:
  1. systemctl enable firewalld
  2. systemctl start firewalld

⚠️  Warning: This fix may affect network connectivity

Apply this fix? [y/N/d/s/?] n

✗ Skipped

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Summary: 1 applied, 1 skipped, 1 no fix available
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Fix safety levels:**

Controls can declare fix safety levels in their definitions:

```yaml
controls:
  items:
    - id: ssh-root-disabled
      fix:
        safety: safe              # Low risk
        commands:
          - sed -i 's/PermitRootLogin yes/PermitRootLogin no/g' /etc/ssh/sshd_config
          - systemctl reload sshd

    - id: firewall-enabled
      fix:
        safety: warning           # Shows warning in prompt
        warning: "May affect network connectivity"
        commands:
          - systemctl enable firewalld
          - systemctl start firewalld

    - id: disk-encryption
      fix:
        safety: dangerous         # Requires --force, never auto-applied
        warning: "Requires reboot, may cause data loss if interrupted"
```

| Safety Level | Interactive | `--yes` | `--yes-safe-only` | `--force` |
|--------------|-------------|---------|-------------------|-----------|
| `safe` | Prompt | Auto-apply | Auto-apply | Auto-apply |
| `warning` | Prompt ⚠️ | Auto-apply | **Skip** | Prompt |
| `dangerous` | Prompt 🛑 | **Skip** | **Skip** | Prompt |

**Recommended for CI/CD:**

```bash
# RECOMMENDED: Only auto-apply safe fixes, skip risky ones
reglet fix profile.yaml --yes-safe-only

# DANGEROUS: Auto-applies safe AND warning fixes (not recommended)
reglet fix profile.yaml --yes

# Why --yes-safe-only matters:
# - "warning" fixes may have side effects (restart services, modify network)
# - CI environments can't interactively confirm risky operations
# - Failed fixes in CI can cause cascading deployment failures
# - Safe fixes are low-risk: config changes, permission updates
```

**Rollback support:**

```bash
# Enable automatic rollback on failure
reglet fix profile.yaml --rollback

# If fix fails mid-way:
# ✗ Fix failed at step 2, rolling back...
# ✓ Rollback complete - system restored to previous state
```

**plugin** - Plugin management
```bash
reglet plugin list
reglet plugin install reglet/aws@1.0
reglet plugin install ./custom-plugin.wasm
reglet plugin init my-plugin --lang rust
reglet plugin build ./my-plugin
reglet plugin test ./my-plugin
```

**test** - Unit test profiles with mocked data

Test your profiles without hitting real infrastructure. Mock plugin responses to verify expect expressions and control logic.

```bash
# Run all tests for a profile
reglet test profile.yaml

# Run specific test file
reglet test profile.yaml --tests tests/profile.test.yaml

# Run with verbose output
reglet test profile.yaml --verbose
```

**Test file format:**

```yaml
# tests/production-baseline.test.yaml
tests:
  # Test that bad config fails
  - name: "SSH root login enabled should fail"
    control: ssh-root-disabled
    mock:
      plugin: file
      config:
        path: /etc/ssh/sshd_config
      response:
        exists: true
        content: |
          Port 22
          PermitRootLogin yes
          PasswordAuthentication no
    assert:
      status: failed
      # Can also assert on specific expect failures
      failed_expects:
        - 'content contains "PermitRootLogin no"'

  # Test that good config passes
  - name: "SSH root login disabled should pass"
    control: ssh-root-disabled
    mock:
      plugin: file
      config:
        path: /etc/ssh/sshd_config
      response:
        exists: true
        content: |
          Port 22
          PermitRootLogin no
          PasswordAuthentication no
    assert:
      status: passed

  # Test HTTP health check with various responses
  - name: "API returning 500 should fail"
    control: api-health
    mock:
      plugin: http
      config:
        url: https://api.example.com/health
      response:
        status_code: 500
        body: "Internal Server Error"
        response_time_ms: 50
    assert:
      status: failed

  - name: "API slow response should fail"
    control: api-health
    mock:
      plugin: http
      response:
        status_code: 200
        body: '{"status": "healthy"}'
        response_time_ms: 5000    # Exceeds threshold
    assert:
      status: failed
      failed_expects:
        - "response_time_ms < 500"

  # Test loop behavior
  - name: "One failing service in loop should fail control"
    control: services-reachable
    mock:
      - plugin: tcp
        config: { host: "db-1.example.com", port: 5432 }
        response: { connected: true }
      - plugin: tcp
        config: { host: "db-2.example.com", port: 5432 }
        response: { connected: false, error: "connection refused" }
    assert:
      status: failed
      loop:
        total: 2
        passed: 1
        failed: 1
```

**Test output:**

```
$ reglet test profile.yaml

Running 5 tests from tests/production-baseline.test.yaml...

✓ SSH root login enabled should fail (2ms)
✓ SSH root login disabled should pass (1ms)
✓ API returning 500 should fail (1ms)
✗ API slow response should fail (2ms)
  Expected: status == failed
  Got: status == passed

  Hint: Check expect expression "response_time_ms < 500"
        Mock provided response_time_ms: 5000
        Expression evaluated to: true (BUG: check your mock data)

✓ One failing service in loop should fail control (3ms)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results: 4 passed, 1 failed
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Why this matters:**
- Test complex regex patterns without creating files
- Verify edge cases (timeouts, errors, partial failures)
- CI/CD validation of profile changes
- Test loop aggregation logic
- Document expected behavior

**catalog** - Catalog management
```bash
reglet catalog list
reglet catalog show iso27001
reglet catalog search "access control"
```

**poam** - Generate Plan of Action & Milestones
```bash
reglet poam profile.yaml --output oscal
```

**update** - Self-update the CLI binary
```bash
reglet update
# Checking for updates...
# Current: v1.0.0
# Latest:  v1.1.0
#
# Changelog:
#   - Added GCP Workload Identity support
#   - Fixed retry backoff calculation
#   - New plugin: azure@1.0
#
# Download and install? [Y/n] y
# Downloading v1.1.0...
# ✓ Updated successfully
# Restart your shell or run: hash -r

# Check for updates without installing
reglet update --check

# Force update (skip confirmation)
reglet update --yes

# Update to specific version
reglet update --version 1.0.5

# Downgrade (requires --force)
reglet update --version 0.9.0 --force
```

### Global Flags

```bash
--config <path>       # Path to system config
--context <name>      # Execution context (e.g., staging, production)
--output <format>     # Output format: table, json, yaml, sarif, oscal-ar, junit
--quiet               # Suppress non-essential output
--verbose             # Verbose output
--no-color            # Disable colored output
--offline             # No network - use only embedded/cached plugins
--auth-oidc           # Force OIDC authentication (even if not auto-detected)
--auth-oidc-required  # Fail if OIDC unavailable (use in CI to prevent secret fallback)
--auth-static         # Force static credentials (skip OIDC even if available)
--timeout <duration>  # Global execution timeout (default: 10m). Examples: 30s, 5m, 1h
--parallelism <n>     # Max concurrent controls (default: NumCPU)

# Filtering (run subset of controls)
--tags <list>         # Run only controls with specific tags (comma-separated)
--skip-tags <list>    # Exclude controls with specific tags
--severity <level>    # Run only controls >= severity (critical, high, medium, low)
--control <id>        # Run specific control(s) by ID (comma-separated)
--include-dependencies  # Include dependencies of filtered controls (even if they don't match filter)
```

**Filtering examples:**

```bash
# Run only critical and high severity controls
reglet check profile.yaml --severity high

# Run only security-tagged controls
reglet check profile.yaml --tags security

# Run security controls but skip slow network checks
reglet check profile.yaml --tags security --skip-tags network,slow

# Run specific control (useful after fixing an issue)
reglet check profile.yaml --control ssh-root-disabled

# Combine filters (AND logic)
reglet check profile.yaml --severity critical --tags production
```

### Offline Mode

The `--offline` flag disables all network operations. This is useful for air-gapped environments, CI caching, or ensuring reproducible builds.

**What works offline:**

| Feature | Offline | Notes |
|---------|---------|-------|
| Embedded plugins | ✓ | Built into binary |
| Cached plugins | ✓ | Previously downloaded |
| Env var secrets | ✓ | No network needed |
| File secrets | ✓ | Local filesystem |
| Local file checks | ✓ | `plugin: file` |
| Local process checks | ✓ | `plugin: process`, `plugin: systemd` |

**What fails offline:**

| Feature | Offline | Error |
|---------|---------|-------|
| Non-cached plugins | ✗ | "Plugin 'aws@1.0' not in cache" |
| HTTP checks | ✗ | Observation marked as `error` |
| TCP/DNS checks | ✗ | Observation marked as `error` |
| Vault/external secrets | ✗ | "Secret source unavailable offline" |
| Cloud auth (OIDC) | ✗ | "OIDC requires network" |
| Cloud auth (SDK) | ✗ | "Cannot refresh credentials offline" |
| Remote profile extends | ✗ | "Cannot fetch remote profile offline" |

**Cloud credentials and offline:**

```bash
# This will FAIL offline (needs to call AWS STS)
reglet check aws-profile.yaml --offline
# Error: AWS plugin requires network for credential refresh

# This WORKS offline if credentials are pre-cached
AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
  reglet check aws-profile.yaml --offline
# Warning: Using static credentials (cannot verify they're valid)

# Pre-cache credentials for offline use
reglet auth cache aws --duration 1h
reglet check aws-profile.yaml --offline  # Uses cached credentials
```

**Offline-first profile design:**

```yaml
# offline-ready.yaml - only uses local checks
profile:
  name: offline-baseline
  version: 1.0.0

plugins:
  - file        # Embedded, works offline
  - systemd     # Embedded, works offline
  - process     # Embedded, works offline
  # - http      # Would fail offline - don't include

controls:
  items:
    - id: ssh-config
      observations:
        - plugin: file
          config:
            path: /etc/ssh/sshd_config
          expect:
            - exists == true
```

**Recommended CI workflow for offline:**

```bash
# Step 1: Warm the cache (with network)
reglet plugin download aws@1.0 http@1.0

# Step 2: Run checks (can be offline)
reglet check profile.yaml --offline
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All controls passed |
| 1 | One or more controls failed |
| 2 | Configuration or execution error |

---

## 9. Open Source Features

### Core Engine
- CLI with all commands
- WASM runtime (wazero)
- Profile loading with defaults/inheritance
- Variable resolution and templating
- Observation execution and expect evaluation
- Evidence collection
- All output formats (Table, JSON, YAML, SARIF, OSCAL)
- **Stateless design**: No database or storage requirements
- **Diff comparison**: `--diff previous.json` for regression detection (user manages storage)

### Built-in Plugins (Embedded)
- **Embedded in binary** (zero setup): file, command, http, tcp, dns, smtp, cert, process, systemd
- Additional via registry: secrets, container, sbom, aws, gcp, azure
- Offline mode works with embedded plugins only

### Developer Experience
- `reglet init` wizard
- `reglet explain` documentation
- `reglet fix` auto-remediation
- `.regletignore` waivers (suppress known issues with documented reasons, scoped by target/context)
- Clear error messages with remediation hints

### Plugin Authoring
- Full Plugin SDK (Rust, Go, AssemblyScript)
- Plugin scaffolding, testing, building
- Community plugin registry
- Complete documentation and examples

### Integrations
- GitHub Action (official)
- GitLab CI template
- Pre-commit hook
- VS Code extension
- **OIDC Authentication**: Native support for CI/CD OIDC token exchanges (GitHub Actions, GitLab CI, CircleCI, Buildkite)
- **Cloud OIDC**: AWS STS, GCP Workload Identity, Azure federated credentials

### IaC Plugins
- Terraform plan/state validation
- Kubernetes manifest checks
- Dockerfile best practices

### Community Catalogs
- Basic CIS benchmarks
- Common infrastructure checks
- Community-contributed controls

---

## 10. Enterprise Features

### Core Enterprise

| Feature | Description |
|---------|-------------|
| Complete Catalogs | Full ISO27001, SOC2, FedRAMP, NIST 800-53, PCI-DSS, HIPAA |
| Web Dashboard | Central visibility, trends, team management |
| API Access | REST/GraphQL API for automation |
| Evidence Vault | Managed S3/GCS storage with retention policies |
| Exception Management | Waiver workflow with approval chain |
| Historical Trending | Managed result storage, track compliance over time, regression alerts |
| Import Merge | `--merge` mode for import: intelligently merge new resources while preserving customizations |
| Multi-Tenant | Organization/team isolation |
| SSO/SCIM | SAML, OIDC, user provisioning |
| RBAC | Fine-grained access control |
| PDF Reports | Auditor-ready compliance reports |
| Support | Dedicated support with SLA |

### Enterprise Integrations

| Integration | Description |
|-------------|-------------|
| SIEM | Splunk, Datadog, Elastic, Sumo Logic |
| Ticketing | Jira, ServiceNow, PagerDuty |
| Notifications | Slack, Microsoft Teams, email |
| Cloud | Native AWS, GCP, Azure integrations |

### Advanced Scanning (Enterprise)

**Cloud API Scanning (CSPM)**
- IAM policy analysis
- S3/GCS bucket configuration
- Security group validation
- Cross-account scanning

**Agentless Snapshot Scanning**
- EBS/GCP PD/Azure Disk snapshots
- Deep VM analysis without agents
- Point-in-time evidence for auditors
- Package, config, and secret scanning

### AI Features (Enterprise)

| Feature | Description |
|---------|-------------|
| Gap Analysis | "You're SOC2 compliant, here's what you need for ISO27001" |
| Evidence Summarization | Human-readable narratives from raw observations |
| Policy Generation | Describe your stack, get a baseline profile |
| Remediation Suggestions | Context-aware fix recommendations |
| Local LLM Support | On-prem AI for sensitive environments |

---

## 11. Project Structure

```
reglet/
├── cmd/
│   └── reglet/              # CLI entry point
│       ├── main.go
│       ├── check.go
│       ├── init.go
│       ├── explain.go
│       ├── fix.go
│       ├── plugin.go
│       ├── catalog.go
│       └── poam.go
│
├── internal/
│   ├── config/                  # Profile and system config loading
│   │   ├── profile.go
│   │   ├── system.go
│   │   ├── defaults.go
│   │   └── variables.go
│   │
│   ├── engine/                  # Execution orchestration
│   │   ├── engine.go
│   │   ├── executor.go
│   │   ├── dependencies.go
│   │   └── results.go
│   │
│   ├── wasm/                    # WASM runtime (wazero)
│   │   ├── runtime.go
│   │   ├── plugin.go
│   │   └── host.go
│   │
│   ├── capabilities/            # Capability enforcement
│   │   ├── grants.go
│   │   ├── filesystem.go
│   │   ├── network.go
│   │   └── exec.go
│   │
│   ├── expect/                  # Expect expression evaluation
│   │   ├── evaluator.go
│   │   ├── compiler.go
│   │   └── functions.go
│   │
│   ├── evidence/                # Evidence collection
│   │   ├── collector.go
│   │   ├── artifact.go
│   │   └── hash.go
│   │
│   ├── oscal/                   # OSCAL generation
│   │   ├── assessment.go
│   │   ├── poam.go
│   │   └── types.go
│   │
│   ├── catalogs/                # Catalog loading
│   │   ├── loader.go
│   │   ├── registry.go
│   │   └── search.go
│   │
│   ├── fix/                     # Auto-remediation
│   │   ├── fixer.go
│   │   └── strategies.go
│   │
│   ├── output/                  # Output formatters
│   │   ├── table.go
│   │   ├── json.go
│   │   ├── yaml.go
│   │   ├── sarif.go
│   │   └── oscal.go
│   │
│   └── version/                 # Version and branding
│       └── version.go
│
├── plugins/                     # Built-in plugin source
│   ├── file/
│   ├── command/
│   ├── http/
│   ├── tcp/
│   ├── dns/
│   ├── smtp/
│   ├── cert/
│   ├── process/
│   ├── systemd/
│   ├── secrets/
│   └── container/
│
├── sdk/                         # Plugin SDK
│   ├── rust/
│   ├── go/
│   └── assemblyscript/
│
├── catalogs/                    # OSS catalog definitions
│   ├── cis/
│   └── community/
│
├── integrations/                # CI/IDE integrations
│   ├── github-action/
│   ├── gitlab-ci/
│   ├── pre-commit/
│   └── vscode/
│
├── wit/                         # WASM Interface Types
│   └── reglet.wit
│
├── docs/                        # Documentation
│   ├── getting-started.md
│   ├── configuration.md
│   ├── plugins.md
│   └── catalogs.md
│
└── examples/                    # Example profiles
    ├── basic-infrastructure.yaml
    ├── production-readiness.yaml
    └── compliance-baseline.yaml
```

---

## 12. Implementation Phases

### Phase 1a: MVP

**Goal**: Ship something useful - single command that runs checks and shows results

*The "hello world" - prove the architecture works end-to-end.*

- [ ] WASM runtime integration (wazero)
- [ ] Capability system and enforcement
- [ ] Profile loading with defaults
- [ ] Variable resolution and templating
- [ ] **Embedded plugins** (Go embed): file, command, http, tcp, dns, smtp, cert, process, systemd
- [ ] Plugin resolution: embedded → cache → registry
- [ ] Expect expression evaluation
- [ ] **Pre-flight validation** - validate plugin configs against schemas before execution
- [ ] Basic CLI: `reglet check`
- [ ] Output: Table, JSON, YAML, JUnit XML
- [ ] **Single binary distribution** - no external dependencies, works offline with embedded plugins

### Phase 1b: Core Complete

**Goal**: Production-ready engine with all core features

*The "engine block" - everything needed to run reliably at scale.*

- [ ] **Profile inheritance** - `extends:` for config loader to merge YAMLs before parsing
- [ ] **Tag & severity filtering** - engine filters execution graph before running
- [ ] **Global timeout** - context management in root check command
- [ ] Parallel execution with leveled topological sort
- [ ] **Rate limiting** - global, per-plugin, and per-observation limits with 429 backoff
- [ ] **Retry/backoff** - configurable retries for transient failures (network timeouts, 429s)
- [ ] **Evidence redaction** - auto-detect and redact secrets in output (built-in patterns)
- [ ] **Evidence size limits** - truncate raw content at 1MB, binary detection
- [ ] **ReDoS protection** - validate user regex patterns, timeout on match
- [ ] **Lockfile generation** - `reglet.lock` with checksums for reproducible builds
- [ ] Lockfile verification on `reglet check`

### Phase 2: Developer Experience

**Goal**: Make the tool easy to adopt and extend

*The "adoption drivers" - solve the empty room problem, enable TDD for compliance.*

- [ ] `reglet init` interactive wizard
- [ ] **`reglet import`** ⭐ - generate profile from local system (the "instant value" moment)
- [ ] `reglet plan` - dry-run/validation mode
- [ ] `reglet validate` - fast schema-only validation for IDE/pre-commit
- [ ] `reglet explain` documentation
- [ ] **`reglet fix`** - auto-remediation with safety levels (safe/risky/dangerous)
- [ ] **Auth credential caching** - `reglet auth cache clear`, token refresh management
- [ ] **Offline mode** - `--offline` flag, credential caching for air-gapped environments
- [ ] `.regletignore` waivers support
- [ ] Remediation hints in output
- [ ] Plugin SDK (Rust, Go)
- [ ] `reglet plugin` commands (init, build, test, install)
- [ ] **`reglet test`** ⭐ - unit test profiles with mocked responses (TDD for compliance)
- [ ] Secret scanning plugin
- [ ] Output attestation (signing with local keys)
- [ ] **OIDC authentication** - GitHub Actions, GitLab CI (shift-left needs this early, not Phase 4)
- [ ] **`reglet pack install <git-url>`** - basic pack support from Git repos (community sharing starts now)
- [ ] **JSON Schema publishing** - publish profile schema to reglet.dev/schemas/

### Phase 3: OSCAL & Evidence

**Goal**: Compliance-ready output and evidence collection

*The "compliance payoff" - not needed for beta users, but needed for first paying customer.*

- [ ] Evidence collection and artifact storage
- [ ] OSCAL Assessment Results output
- [ ] OSCAL POA&M generation
- [ ] SARIF output for IDE/CI integration
- [ ] `reglet poam` command
- [ ] Findings aggregation and severity handling
- [ ] **Profile templating** - reusable control templates with parameterization

### Phase 4: Integrations

**Goal**: Seamless CI/CD and IaC integration

*The "ecosystem" - deep integrations after core OIDC lands in Phase 2.*

- [ ] GitHub Action (official, marketplace)
- [ ] GitLab CI template
- [ ] CircleCI, Buildkite OIDC support (extending Phase 2 OIDC)
- [ ] GCP Workload Identity Federation integration
- [ ] Azure federated credentials integration
- [ ] Pre-commit hook
- [ ] VS Code extension
- [ ] Terraform plan/state plugin
- [ ] Kubernetes manifest plugin
- [ ] Dockerfile plugin
- [ ] Container image scanning
- [ ] SBOM generation
- [ ] `reglet import aws` - generate from AWS account
- [ ] `reglet import k8s` - generate from Kubernetes cluster
- [ ] `reglet import terraform` - generate from Terraform state

### Phase 5: Enterprise Foundation

**Goal**: Core enterprise features

- [ ] Web dashboard (React/Next.js)
- [ ] REST API
- [ ] Evidence vault (S3/GCS)
- [ ] Authentication (SSO/OIDC)
- [ ] Multi-tenant support
- [ ] Complete catalog libraries
- [ ] Exception/waiver workflow
- [ ] PDF report generation

### Phase 6: Advanced Scanning (Enterprise)

**Goal**: Deep cloud and agentless scanning

- [ ] Full cloud API scanning (AWS, GCP, Azure)
- [ ] IAM policy analysis
- [ ] Agentless snapshot scanning
- [ ] Historical trending and dashboards

### Phase 7: AI Features (Enterprise)

**Goal**: AI-assisted compliance

- [ ] Compliance Gap Analysis
- [ ] Evidence Summarization
- [ ] Policy Generation wizard
- [ ] Remediation Suggestions
- [ ] Local LLM support (Ollama)

### Phase 8: Compliance Packs Registry (Formal Marketplace)

**Goal**: Formal registry, verified publishers, monetization

*Phase 2 added basic `pack install <git-url>`. This phase adds the formal ecosystem.*

**The problem with catalogs:**
- "ISO27001" is too abstract - 114 controls, which apply to my AWS + Kubernetes stack?
- Users want: "ISO27001 for AWS + Kubernetes" not raw control lists

**Solution: Compliance Packs**

Packs are community-maintained, stack-specific profiles that combine:
- Framework requirements (ISO27001, SOC2, CIS)
- Infrastructure targets (AWS, GCP, Kubernetes, Linux)
- Best practice defaults (severity, owners, remediation)

```bash
# Browse available packs
reglet pack search aws
# PACK                          FRAMEWORK    TARGET       STARS
# aws-cis-level-1               CIS          AWS          ★★★★★ (2.3k)
# aws-soc2-saas                 SOC2         AWS          ★★★★☆ (892)
# aws-hipaa-healthcare          HIPAA        AWS          ★★★★☆ (456)
# aws-iso27001-startup          ISO27001     AWS          ★★★☆☆ (234)

# Install a pack
reglet pack install aws-cis-level-1
# ✓ Downloaded aws-cis-level-1@2.1.0
# ✓ Created profiles/aws-cis-level-1.yaml
# ✓ Added required plugins to reglet.lock
#
# Next steps:
#   1. Review profiles/aws-cis-level-1.yaml
#   2. Customize severity/owners for your org
#   3. Run: reglet check profiles/aws-cis-level-1.yaml

# Install multiple packs
reglet pack install k8s-production-hardening aws-cis-level-1

# Update packs
reglet pack update
```

**Pack repository structure:**

```
aws-cis-level-1/
├── pack.yaml                 # Pack metadata
├── profile.yaml              # Main profile
├── controls/                 # Control definitions (can be split)
│   ├── iam.yaml
│   ├── s3.yaml
│   ├── ec2.yaml
│   └── network.yaml
├── plugins/                  # Custom plugins (if any)
├── docs/
│   ├── README.md
│   └── controls.md           # Control documentation
└── tests/
    └── profile_test.yaml     # Pack validation tests
```

**When to split controls into separate files:**

| Scenario | Recommendation |
|----------|----------------|
| < 20 controls, single domain | **Inline** - keep in `profile.yaml` |
| 20-50 controls, single domain | **Either** - personal preference |
| > 50 controls | **Split** - organize by domain/service |
| Multiple teams own different controls | **Split** - enables clear ownership |
| Controls are reused across packs | **Split** - share via imports |
| Single-purpose pack (one service) | **Inline** - simpler distribution |

**Inline example (small pack):**

```yaml
# profile.yaml - everything in one file
profile:
  name: simple-linux-baseline
  version: 1.0.0

controls:
  items:
    - id: ssh-root-disabled
      # ...
    - id: firewall-enabled
      # ...
    # 15 more controls inline
```

**Split example (large pack):**

```yaml
# profile.yaml - imports separate files
profile:
  name: aws-cis-level-1
  version: 2.1.0

imports:
  - controls/iam.yaml      # 25 IAM controls
  - controls/s3.yaml       # 18 S3 controls
  - controls/ec2.yaml      # 30 EC2 controls
  - controls/network.yaml  # 22 VPC/SG controls
```

```yaml
# controls/iam.yaml
controls:
  defaults:
    tags: [iam, identity]
    owner: security-team

  items:
    - id: iam-root-mfa
      name: Root account has MFA enabled
      # ...
    # 24 more IAM controls
```

**pack.yaml:**

```yaml
pack:
  name: aws-cis-level-1
  version: 2.1.0
  description: CIS AWS Foundations Benchmark Level 1
  author: Reglet Community
  license: Apache-2.0

  framework: cis-aws-v1.5
  target: aws

  # Pack can depend on other packs
  dependencies:
    - aws-base@1.0

  # Required plugins
  plugins:
    - reglet/aws@1.0
    - reglet/http@1.0

  # What this pack covers
  coverage:
    cis-aws-v1.5:
      level-1: 95%        # Percentage of Level 1 controls
      level-2: 0%         # See aws-cis-level-2 pack

  tags: [aws, cis, security, cloud]

  # Minimum Reglet version
  requires: ">=1.2.0"
```

**Pack registry:**

| Tier | Source | Curation |
|------|--------|----------|
| Official | `packs.reglet.dev` | Maintained by Reglet team |
| Verified | `packs.reglet.dev` | Community, reviewed for quality |
| Community | GitHub/GitLab | Anyone, unreviewed |

```bash
# Install from official registry (default)
reglet pack install aws-cis-level-1

# Install from GitHub
reglet pack install github:myorg/my-compliance-pack

# Install from local path
reglet pack install ./internal-packs/our-standards
```

**Enterprise pack features:**

| Feature | OSS | Enterprise |
|---------|-----|------------|
| Install public packs | ✓ | ✓ |
| Create custom packs | ✓ | ✓ |
| Private pack registry | - | ✓ |
| Pack usage analytics | - | ✓ |
| Pack compliance reports | - | ✓ |
| Org-wide pack policies | - | ✓ |

**Implementation:**

- [ ] Pack manifest format (`pack.yaml`)
- [ ] `reglet pack` CLI commands (search, install, update, list)
- [ ] Official pack registry (packs.reglet.dev)
- [ ] GitHub/GitLab pack sources
- [ ] Pack dependency resolution
- [ ] Pack versioning and updates
- [ ] Community contribution guidelines
- [ ] Verified publisher program

---

## 13. Appendix: Examples

### Basic Infrastructure Validation

```yaml
profile:
  name: infrastructure-baseline
  version: 1.0.0

plugins:
  - reglet/file@1.0
  - reglet/tcp@1.0
  - reglet/http@1.0

controls:
  defaults:
    severity: warning
    owner: platform-team

  items:
    - id: disk-space
      name: Adequate Disk Space
      severity: critical
      observations:
        - plugin: command
          config:
            run: df -h / | tail -1 | awk '{print $5}' | tr -d '%'
          expect:
            - int(stdout) < 80

    - id: api-responding
      name: API Health Check
      observations:
        - plugin: http
          config:
            url: https://api.example.com/health
            timeout: 5s
          expect:
            - status_code == 200
            - response_time_ms < 500
```

### Production Readiness Gate

```yaml
profile:
  name: production-readiness
  version: 1.0.0
  description: Must pass before deploying to production

plugins:
  - reglet/http@1.0
  - reglet/tcp@1.0
  - reglet/dns@1.0

vars:
  services:
    - name: api
      host: api.example.com
      port: 443
    - name: auth
      host: auth.example.com
      port: 443
    - name: db
      host: db.example.com
      port: 5432

controls:
  defaults:
    severity: critical
    owner: sre-team
    tags: [production, deploy-gate]

  items:
    - id: services-reachable
      name: All Services Reachable
      observations:
        - loop:
            items: "{{ .vars.services }}"
          plugin: tcp
          config:
            host: "{{ .loop.item.host }}"
            port: "{{ .loop.item.port }}"
          expect:
            - connected == true

    - id: dns-resolves
      name: DNS Records Valid
      observations:
        - loop:
            items: "{{ .vars.services }}"
          plugin: dns
          config:
            name: "{{ .loop.item.host }}"
            type: A
          expect:
            - resolved == true
```

### Compliance Baseline (ISO27001)

```yaml
profile:
  name: iso27001-baseline
  version: 1.0.0

catalogs:
  - iso27001

plugins:
  - reglet/file@1.0
  - reglet/command@1.0
  - reglet/systemd@1.0

controls:
  defaults:
    severity: high
    owner: security-team
    tags: [compliance, iso27001]

  items:
    - id: a.9.4.2-ssh-root
      name: Disable SSH Root Login
      mappings:
        iso27001: A.9.4.2
      observations:
        - plugin: file
          config:
            path: /etc/ssh/sshd_config
          expect:
            - content matches "PermitRootLogin\\s+no"
      remediation: |
        Set `PermitRootLogin no` in /etc/ssh/sshd_config

    - id: a.12.4.1-audit-logging
      name: Audit Logging Enabled
      mappings:
        iso27001: A.12.4.1
      observations:
        - plugin: systemd
          config:
            unit: auditd.service
          expect:
            - active == true
            - enabled == true
      remediation: |
        Enable auditd: `systemctl enable --now auditd`

    - id: a.13.1.1-firewall
      name: Firewall Active
      mappings:
        iso27001: A.13.1.1
      observations:
        - plugin: systemd
          config:
            unit: firewalld.service
          expect:
            - active == true
      remediation: |
        Enable firewalld: `systemctl enable --now firewalld`
```

---

## Design Principles

1. **Declarative First**: Configuration over code for policies
2. **OSCAL Native**: Built around compliance standards from the start
3. **Plugin Everything**: Core is thin; capabilities come from WASM plugins
4. **Secure by Default**: Sandboxed plugins, explicit capability grants
5. **Evidence Native**: Every observation captures proof for auditors
6. **Shift Left**: Designed for CI/CD, pre-commit, IDE integration
7. **Open Core**: Generous open source with sustainable enterprise model
8. **Easy Renaming**: Branding centralized for future changes

---

*Document generated for Reglet v1.0*
