# Security Model

Reglet implements a **capability-based security model** with automatic permission discovery and enforcement.

## Core Principles

1. **Least Privilege**: Plugins can only access resources (files, network, environment) if explicitly granted
2. **Sandboxed Execution**: All validation logic runs in a CGO-free WebAssembly runtime (wazero)
3. **Automatic Discovery**: Permissions are extracted from profile configuration, not requested broadly

## Profile-Based Capability Discovery

When you run a profile, Reglet analyzes your observation configs to extract the **minimum required permissions**:

```yaml
observations:
  - plugin: file
    config:
      path: /etc/ssh/sshd_config  # Only requests: fs:read:/etc/ssh/sshd_config
```

Instead of requesting broad access like `read:**` (all files), Reglet requests only what your profile needs.

## Security Governance Levels

Control how Reglet handles capability requests with the `--security` flag:

| Mode | Behavior | Target Environment |
|:-----|:---------|:-------------------|
| **strict** | Automatically **denies** broad patterns (e.g., `/bin/bash` or `read:/**`) | Production CI/CD |
| **standard** | **Warns and prompts** the user with risk descriptions | Local development (default) |
| **permissive** | **Auto-grants** all requested capabilities | Trusted local automation |

### Broad Capability Detection

Reglet identifies certain patterns as high-risk "broad" capabilities:

- **Shell interpreters**: `/bin/bash`, `/bin/sh`, `/bin/zsh`
- **Script interpreters**: `python`, `perl`, `node`, `ruby`
- **Wildcard paths**: `fs:read:/**`, `fs:write:/tmp/**`

These are flagged because they can bypass resource restrictions via arbitrary code execution.

## Configuration

Set your preferred security level in `~/.reglet/config.yaml`:

```yaml
security:
  level: standard  # strict, standard, or permissive
  custom_broad_patterns:  # Optional: define additional broad patterns
    - "fs:write:/tmp/**"
```

Command-line flags override config file settings:

```bash
./bin/reglet check --security=strict profile.yaml
```

## Capability Types

| Type | Format | Example |
|:-----|:-------|:--------|
| **Filesystem** | `fs:read:<path>` or `fs:write:<path>` | `fs:read:/etc/passwd` |
| **Network** | `net:<protocol>:<host>:<port>` | `net:tcp:example.com:443` |
| **DNS** | `dns:resolve:<domain>` | `dns:resolve:example.com` |
| **Execute** | `exec:<path>` | `exec:/usr/bin/systemctl` |

## WASM Sandbox

All plugins run inside WebAssembly with:

- **Memory isolation**: Each plugin has its own linear memory space
- **No direct syscalls**: All host interactions go through capability-checked host functions
- **Resource limits**: Configurable memory limits prevent denial-of-service

## Path Traversal Prevention

Reglet validates all paths to prevent:

- Symlink escapes
- `../` traversal attacks
- Absolute path canonicalization bypasses

File access uses `os.OpenRoot` (Go 1.24+) for sandboxed filesystem operations.

## Secret Redaction

Plugin output is automatically scanned for:

- API keys and tokens
- Passwords and secrets
- Private keys

Detected secrets are redacted or hashed before appearing in reports.
