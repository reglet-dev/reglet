# ![logov1-small](https://github.com/user-attachments/assets/d62c9bd1-d769-4775-a9d4-1871d4be8f74) Reglet

> **Compliance as Code. Secure by Design.**

Reglet is a compliance and infrastructure validation engine that runs security checks in isolated WebAssembly sandboxes. Define policies as code, validate systems and services, and get standardized audit output—ready for SOC2, ISO27001, and more.

[![Build Status](https://github.com/whiskeyjimbo/reglet/workflows/CI/badge.svg)](https://github.com/whiskeyjimbo/reglet/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/whiskeyjimbo/reglet)](https://goreportcard.com/report/github.com/whiskeyjimbo/reglet)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Quick Start (30 seconds)

```bash
# Clone and build
git clone https://github.com/whiskeyjimbo/reglet.git
cd reglet
make build

# Try it
./bin/reglet check docs/examples/01-quickstart.yaml

# Example output:
# ✓ root-home-permissions - Root home directory permissions
# ✓ shadow-file-permissions - Shadow file is not world-readable
# ✓ tmp-directory-exists - Temporary directory exists and is writable
#
# 3 passed, 0 failed
```

**Prerequisites:** Go 1.25+, Make

## Features

- **Declarative Profiles** — Define validation rules in simple, versioned YAML
- **Parallel Execution** — Optimized for CI/CD with concurrent execution of independent controls
- **Standardized Output** — JSON, YAML, JUnit, SARIF—ready for compliance platforms or OSCAL integration (coming soon)
- **Secure Sandbox** — All validation logic runs inside a CGO-free WebAssembly runtime (wazero)
- **Capability-Based Security** — Plugins can only access files, networks, or environment variables if explicitly allowed
- **Automatic Redaction** — Sensitive data (secrets, tokens) is automatically detected and redacted before reporting

## What Can It Validate?

| Plugin | Use Case |
|:-------|:---------|
| **file** | Permissions, ownership, content patterns |
| **command** | Exit codes, output content |
| **http** | HTTP/HTTPS endpoints, response validation |
| **dns** | DNS records and resolution |
| **tcp** | Port connectivity, TLS certificates |
| **smtp** | Mail server connectivity |

See [examples/](docs/examples/) for working profiles.

## Security Model

Reglet uses **capability-based security**—plugins can only access what's explicitly granted:

- **Automatic Discovery**: Permissions are extracted from your profile (e.g., `path: /etc/passwd` → only grants read to that file)
- **No Broad Access**: Unlike scripts with full host access, plugins are sandboxed
- **Security Levels**: Control how Reglet handles risky patterns:
  - `strict` — Deny broad capabilities automatically
  - `standard` — Warn and prompt before granting (default)
  - `permissive` — Auto-grant for trusted environments

```yaml
# ~/.reglet/config.yaml
security:
  level: standard  # strict, standard, or permissive
```

See [docs/security.md](docs/security.md) for the full security architecture.

## Example Profile

```yaml
profile:
  name: SSH Security
  description: Check SSH configuration
  version: 1.0.0

controls:
  - id: sshd-config
    name: SSH password authentication disabled
    observations:
      - plugin: file
        config:
          path: /etc/ssh/sshd_config
        expect: |
          data.content.contains("PasswordAuthentication no")
```

## Installation

### From Source

```bash
git clone https://github.com/whiskeyjimbo/reglet.git
cd reglet
make build
./bin/reglet check docs/examples/01-quickstart.yaml
```

### Examples

- **[01-quickstart.yaml](docs/examples/01-quickstart.yaml)** — Basic system security checks
- **[02-ssh-hardening.yaml](docs/examples/02-ssh-hardening.yaml)** — SSH hardening (SOC2 CC6.1)
- **[03-web-security.yaml](docs/examples/03-web-security.yaml)** — HTTP/HTTPS validation
- **[04-dns-validation.yaml](docs/examples/04-dns-validation.yaml)** — DNS resolution and records
- **[05-tcp-connectivity.yaml](docs/examples/05-tcp-connectivity.yaml)** — TCP ports and TLS testing

## Status: Alpha (v0.2.0-alpha)

Reglet is in active development. Core features work, but expect breaking changes before 1.0.

### Roadmap

**v0.2.0-alpha** (Current)
- [x] Core execution engine with parallel execution
- [x] Plugins: File, HTTP, DNS, TCP, Command, SMTP
- [x] Capability system with profile-based discovery
- [x] Configurable security levels (strict/standard/permissive)
- [x] Automatic secret redaction
- [x] Output formatters (Table, JSON, YAML, JUnit, SARIF)

**v0.3.0-alpha**
- [ ] Profile inheritance
- [ ] OSCAL output
- [ ] Binary releases for Linux/macOS/Windows

**v1.0**
- [ ] Cloud provider plugins (AWS, GCP, Azure)
- [ ] Compliance packs (SOC2, ISO27001, FedRAMP)
- [ ] CI/CD integrations (GitHub Actions, GitLab CI)
- [ ] Plugin SDK documentation

## Community

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md).

- **Issues:** [GitHub Issues](https://github.com/whiskeyjimbo/reglet/issues)
- **Discussions:** [GitHub Discussions](https://github.com/whiskeyjimbo/reglet/discussions)

## License

Apache-2.0 — See [LICENSE](LICENSE)
