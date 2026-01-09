# ![logo-small](.github/assets/logo-small.svg) Reglet

> **Compliance as Code. Secure by Design.**

Reglet is a compliance and infrastructure validation engine that runs security checks in isolated WebAssembly sandboxes. Define policies as code, validate systems and services, and get standardized audit output ready for SOC2, ISO27001, and more.

<p align="center">
  <a href="https://github.com/reglet-dev/reglet/actions"><img src="https://github.com/reglet-dev/reglet/workflows/CI/badge.svg" alt="Build Status"></a>
  <a href="https://goreportcard.com/report/github.com/reglet-dev/reglet"><img src="https://goreportcard.com/badge/github.com/reglet-dev/reglet?style=flat" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <img src="https://img.shields.io/github/v/release/reglet-dev/reglet?include_prereleases" alt="Latest Release">
</p>

## Quick Start

```bash
# Install (choose one)
brew install reglet-dev/tap/reglet           # macOS/Linux
docker pull ghcr.io/reglet-dev/reglet:latest # Docker
curl -sSL https://raw.githubusercontent.com/reglet-dev/reglet/main/scripts/install.sh | sh  # Script

# Get an example profile
curl -fsSL https://raw.githubusercontent.com/reglet-dev/reglet/main/docs/examples/01-quickstart.yaml > quickstart.yaml

# Run it
reglet check quickstart.yaml

# Or with Docker
docker run --rm -v $(pwd)/quickstart.yaml:/quickstart.yaml \
  ghcr.io/reglet-dev/reglet:latest check /quickstart.yaml
```
![demo](.github/assets/demo.gif)

## Usage
```bash
# Output formats
reglet check profile.yaml --format=json
reglet check profile.yaml --format=sarif -o results.sarif

# Quiet mode for CI/scripts
reglet check profile.yaml --quiet

# Debug mode
reglet check profile.yaml --log-level=debug

# Filter controls
reglet check profile.yaml --tags security
reglet check profile.yaml --severity critical,high
```

## Features

- **Declarative Profiles** - Define validation rules in simple, versioned YAML
- **Parallel Execution** - Optimized for CI/CD with concurrent execution of independent controls
- **Standardized Output** - JSON, YAML, JUnit, SARIF - ready for compliance platforms or OSCAL integration (coming soon)
- **Secure Sandbox** - All validation logic runs inside a CGO-free WebAssembly runtime (wazero)
- **Capability-Based Security** - Plugins can only access files, networks, or environment variables if explicitly allowed
- **Automatic Redaction** - Sensitive data (secrets, tokens) is automatically detected and redacted before reporting

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

Reglet uses **capability-based security** - plugins can only access what's explicitly granted:

- **Automatic Discovery**: Permissions are extracted from your profile (e.g., `path: /etc/passwd` grants read to only that file)
- **No Broad Access**: Unlike scripts with full host access, plugins are sandboxed
- **Security Levels**: Control how Reglet handles risky patterns:
  - `strict` - Deny broad capabilities automatically
  - `standard` - Warn and prompt before granting (default)
  - `permissive` - Auto-grant for trusted environments

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

plugins:
  - file

controls:
  items:
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

### Homebrew (macOS/Linux)

```bash
brew install reglet-dev/tap/reglet
reglet version
```

### Docker

```bash
# Pull image
docker pull ghcr.io/reglet-dev/reglet:latest

# Quick version check
docker run --rm ghcr.io/reglet-dev/reglet:latest version

# Run with profile from host
docker run --rm -v $(pwd)/profile.yaml:/profile.yaml \
  ghcr.io/reglet-dev/reglet:latest check /profile.yaml

# Try built-in examples
docker run --rm ghcr.io/reglet-dev/reglet:latest \
  check /home/reglet/docs/examples/01-quickstart.yaml
```

### Install Script (Linux/macOS)

```bash
curl -sSL https://raw.githubusercontent.com/reglet-dev/reglet/main/scripts/install.sh | sh
```

### Manual Download

Download the appropriate archive for your platform from the [releases page](https://github.com/reglet-dev/reglet/releases), extract it, and move the binary to your PATH:

```bash
# Linux/macOS
tar -xzf reglet-*.tar.gz
sudo mv reglet /usr/local/bin/
reglet version

# Windows (PowerShell)
Expand-Archive reglet-*.zip
Move-Item reglet.exe C:\Windows\System32\
reglet version
```

### From Source

Requires Go 1.25+:

```bash
git clone https://github.com/reglet-dev/reglet.git
cd reglet
make build
./bin/reglet check docs/examples/01-quickstart.yaml
```

### Examples

- **[01-quickstart.yaml](docs/examples/01-quickstart.yaml)** - Basic system security checks
- **[02-ssh-hardening.yaml](docs/examples/02-ssh-hardening.yaml)** - SSH hardening (SOC2 CC6.1)
- **[03-web-security.yaml](docs/examples/03-web-security.yaml)** - HTTP/HTTPS validation
- **[04-dns-validation.yaml](docs/examples/04-dns-validation.yaml)** - DNS resolution and records
- **[05-tcp-connectivity.yaml](docs/examples/05-tcp-connectivity.yaml)** - TCP ports and TLS testing
- **[06-command-checks.yaml](docs/examples/06-command-checks.yaml)** - Command execution and output validation
- **[07-vars-and-defaults.yaml](docs/examples/07-vars-and-defaults.yaml)** - Variables and control defaults

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
- [x] Binary releases for Linux/macOS/Windows (amd64/arm64)
- [x] Docker images (GHCR multi-arch)
- [x] Homebrew tap
- [x] Automated releases with goreleaser

**v0.3.0-alpha**
- [ ] OCI-based plugin registry (version pinning, aliases)

**v0.4.0-alpha**
- [ ] Profile inheritance
- [ ] OSCAL output

**v1.0**
- [ ] Cloud provider plugins (AWS, GCP, Azure)
- [ ] Compliance packs (SOC2, ISO27001, FedRAMP)
- [ ] CI/CD integrations (GitHub Actions, GitLab CI)
- [ ] Plugin SDK documentation

## Community

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md).

- **Issues:** [GitHub Issues](https://github.com/reglet-dev/reglet/issues)
- **Discussions:** [GitHub Discussions](https://github.com/reglet-dev/reglet/discussions)

## License

Apache-2.0 - See [LICENSE](LICENSE)
