# ![logov1-small](https://github.com/user-attachments/assets/d62c9bd1-d769-4775-a9d4-1871d4be8f74) Reglet

> **Compliance as Code. Secure by Design.**

Reglet is a modern, modular compliance and infrastructure validation engine. It empowers engineering teams to define policy as code, execute validation logic in securely sandboxed WebAssembly (WASM) environments, and generate standardized audit artifacts.

Unlike traditional tools that run scripts with full host privileges, Reglet enforces a strict Capability-Based Security model. Plugins must explicitly request permissions (e.g., "I need to read /etc/passwd" or "I need to connect to port 443"), and these permissions must be granted by the user or policy.

[![Build Status](https://github.com/whiskeyjimbo/reglet/workflows/CI/badge.svg)](https://github.com/whiskeyjimbo/reglet/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/whiskeyjimbo/reglet)](https://goreportcard.com/report/github.com/whiskeyjimbo/reglet)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Table of Contents
- [Core Features](#core-features)
- [Why Reglet?](#why-reglet)
- [Architecture](#architecture)
- [Quick Start](#quick-start-30-seconds)
- [Installation](#installation)
- [Examples](#examples)
- [Community](#community)

## Core Features
- Secure Sandbox: All validation logic runs inside a CGO-free WebAssembly runtime (wazero). Plugins are memory-safe and isolated from the host OS.

- Capability-Based Security: "Least Privilege" is enforced at the system call level. A plugin cannot access files, networks, or environment variables unless explicitly allowed.

- Declarative Profiles: Define validation rules in simple, versioned YAML.

- Automatic Redaction: Sensitive data (secrets, tokens) in plugin output is automatically detected and redacted or hashed before reporting.

- Parallel Execution: Optimized for CI/CD with concurrent execution of independent controls.

- Standardized Output: Generates machine-readable results (JSON, YAML, JUnit, SARIF) ready for compliance platforms or OSCAL integration (future).


## Architecture
Reglet follows an open-core philosophy with a strict focus on security and portability.

- Host: The core engine (Go) manages the lifecycle, capability grants, and reporting. Plugins are securely loaded from the filesystem at runtime.

- SDK: A Go SDK allowing developers to write plugins that compile to WASM/WASI.

- WIT Contracts: The boundary between Host and Plugin is strictly typed using WASM Interface Types (WIT).

## Quick Start (30 seconds)

### Prerequisites
- Go 1.25+
- Make

```bash
# Clone and build
git clone https://github.com/whiskeyjimbo/reglet.git
cd reglet
make build

# Try it
./bin/reglet check examples/01-quickstart.yaml

# Example output:
# ✓ root-home-permissions - Root home directory permissions
# ✓ shadow-file-permissions - Shadow file is not world-readable
# ✓ tmp-directory-exists - Temporary directory exists and is writable
#
# 3 passed, 0 failed
```

## What Does It Check?

**File permissions and content:**
```yaml
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

**Command execution:**
```yaml
controls:
  - id: nginx-running
    name: Nginx is active
    observations:
      - plugin: command
        capabilities:
          - exec:/usr/bin/systemctl
        config:
          command: /usr/bin/systemctl
          args: ["is-active", "nginx"]
        expect: |
          data.exit_code == 0
```

**Also available:** HTTP endpoints, DNS records, TCP ports (see examples/03-05)

## Installation

### From Source (current)

```bash
git clone https://github.com/whiskeyjimbo/reglet.git
cd reglet

# Build core binary
make build

# Build WASM plugins (Required: Reglet loads plugins from the ./plugins directory)
for d in plugins/*/; do (cd "$d" && make build); done
cd ../..

# Run
./bin/reglet check examples/01-quickstart.yaml
```

## Examples

Reglet includes working examples you can try immediately:

- **[01-quickstart.yaml](docs/examples/01-quickstart.yaml)** - Basic system security checks
- **[02-ssh-hardening.yaml](docs/examples/02-ssh-hardening.yaml)** - SSH hardening (SOC2 CC6.1)
- **[03-web-security.yaml](docs/examples/03-web-security.yaml)** - HTTP/HTTPS validation
- **[04-dns-validation.yaml](docs/examples/04-dns-validation.yaml)** - DNS resolution and records
- **[05-tcp-connectivity.yaml](docs/examples/05-tcp-connectivity.yaml)** - TCP ports and TLS testing

## Status: Alpha (v0.2.0-alpha)

Reglet is in active development. Core features work, but expect breaking changes before 1.0.

## Roadmap

**v0.2.0-alpha** (Current)
- ✅ Core execution engine
- ✅ File, HTTP, DNS, TCP and command plugins
- ✅ Capability system
- ✅ Output formatters (Table, JSON, YAML, JUnit, SARIF)

**v0.3.0-alpha**
- Profile inheritance
- OSCAL output
- Binary releases for Linux/macOS/Windows

**v1.0**
- Cloud provider plugins
- compliance packs (SOC2, ISO27001, FedRAMP)
- CI/CD integrations
- Plugin SDK documentation

## Community

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md).

- **Issues:** [GitHub Issues](https://github.com/whiskeyjimbo/reglet/issues)
- **Discussions:** [GitHub Discussions](https://github.com/whiskeyjimbo/reglet/discussions)

## License

Apache-2.0 - See [LICENSE](LICENSE)
