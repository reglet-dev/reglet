# Reglet Examples

These examples demonstrate Reglet's capabilities with real-world checks.

## Quick Start

Try the quickstart example first - it works on any Linux system:

```bash
./bin/reglet check docs/examples/01-quickstart.yaml --trust-plugins
```

## Available Examples

### 01-quickstart.yaml - System Security Basics

**What it checks:**
- Root home directory permissions
- Shadow file permissions
- Temp directory exists
- /etc directory protection

**Requirements:** None - works on any Linux system
**Plugins:** `file`

**Try it:**
```bash
./bin/reglet check examples/01-quickstart.yaml
```

---

### 02-ssh-hardening.yaml - SSH Configuration (SOC2 CC6.1)

**What it checks:**
- SSH config file exists and has correct permissions
- Password authentication disabled
- Root login disabled/restricted
- SSH protocol version 2 enforced
- Empty passwords forbidden
- X11 forwarding disabled

**Requirements:** `/etc/ssh/sshd_config` must exist
**Plugins:** `file`

**Try it:**
```bash
./bin/reglet check examples/02-ssh-hardening.yaml
```

**Filter by severity:**
```bash
./bin/reglet check examples/02-ssh-hardening.yaml --severity critical,high
```

---

### 03-web-security.yaml - Web Server Security

**What it checks:**
- HTTP endpoint accessibility
- HTTPS connectivity and status codes
- Response body validation
- API endpoint testing (GitHub API example)
- JSON response validation

**Requirements:** Network access to test sites
**Plugins:** `http`

**Try it:**
```bash
./bin/reglet check examples/03-web-security.yaml
```

**Custom URL:**
```yaml
# Modify the example to test your own site
controls:
  - id: my-site-check
    observations:
      - plugin: http
        capabilities:
          - network:outbound:443
        config:
          url: https://mysite.com
        expect: data.status_code == 200
```

---

### 04-dns-validation.yaml - DNS Resolution

**What it checks:**
- A records (IPv4 addresses)
- AAAA records (IPv6 addresses)
- CNAME records (aliases)
- MX records (mail exchangers)
- TXT records (verification, SPF)
- NS records (nameservers)
- DNS query performance
- Custom nameserver usage

**Requirements:** Network access for DNS queries
**Plugins:** `dns`

**Try it:**
```bash
./bin/reglet check examples/04-dns-validation.yaml
```

**Check your own domain:**
```bash
# Edit the example to use your domain
sed 's/google.com/yourdomain.com/g' examples/04-dns-validation.yaml > my-dns-check.yaml
./bin/reglet check my-dns-check.yaml
```

---

### 05-tcp-connectivity.yaml - TCP Port Testing

**What it checks:**
- TCP port connectivity (HTTP, HTTPS, SSH, DNS)
- TLS handshake validation
- TLS version verification (1.2, 1.3)
- TLS certificate presence
- Connection performance (response time)
- Service availability

**Requirements:** Network access to test hosts
**Plugins:** `tcp`

**Try it:**
```bash
./bin/reglet check examples/05-tcp-connectivity.yaml
```

**Check your own server:**
```yaml
# Test your server's ports
controls:
  - id: my-server-check
    observations:
      - plugin: tcp
        capabilities:
          - network:outbound:*
        config:
          host: myserver.com
          port: "443"
          tls: true
        expect: |
          data.connected == true &&
          data.tls_version == "TLS 1.3"
```

---

### 06-command-checks.yaml - Command Execution

**What it checks:**
- Service status (systemd)
- System uptime
- Kernel version
- Disk usage thresholds
- Security updates
- User existence

**Requirements:** Linux system with systemd
**Plugins:** `command`

**Try it:**
```bash
./bin/reglet check docs/examples/06-command-checks.yaml --trust-plugins
```

---

### 07-vars-and-defaults.yaml - Variables and Defaults

**What it checks:**
- Demonstrates `vars` substitution using `{{ .vars.key }}` syntax
- Shows `controls.defaults` for inherited severity, owner, tags
- Various file existence checks using variables

**Requirements:** None - works on any Linux system
**Plugins:** `file`

**Try it:**
```bash
./bin/reglet check docs/examples/07-vars-and-defaults.yaml --trust-plugins
```

**Features demonstrated:**
```yaml
vars:
  config_dir: /etc

controls:
  defaults:
    severity: medium
    owner: platform-team
  items:
    - id: example
      # Uses {{ .vars.config_dir }}/passwd
      # Inherits severity=medium, owner=platform-team
```

---

### 20-cli-variable-overrides.yaml - CLI Variable Overrides

**What it demonstrates:**
- Runtime variable override using `--set key=value`
- Reading sensitive values from files with `--set-file`
- Reading values from environment with `--set-env`
- Nested variable override with dot notation (`--set server.host=prod`)
- Type auto-detection (integers, floats, booleans)

**Requirements:** None - demonstrates override capability
**Plugins:** `http`, `file`, `command`

**Try it:**
```bash
# Override environment variable
./bin/reglet check docs/examples/20-cli-variable-overrides.yaml \
  --set environment=prod --trust-plugins

# Override multiple values
./bin/reglet check docs/examples/20-cli-variable-overrides.yaml \
  --set environment=staging \
  --set max_response_time_ms=2000 \
  --set debug_enabled=true \
  --trust-plugins

# Use environment variable (for CI/CD)
export MY_BUILD_ID="abc123"
./bin/reglet check docs/examples/20-cli-variable-overrides.yaml \
  --set-env build_id=MY_BUILD_ID \
  --trust-plugins
```

**Features demonstrated:**
```bash
--set key=value           # Direct value override
--set paths.config=/opt   # Nested path override
--set port=8080           # Auto-detect integer
--set debug=true          # Auto-detect boolean
--set-file api_key=./key  # Read from file (secure)
--set-env token=API_TOKEN # Read from environment variable
--no-warn-unused-vars     # Suppress unused var warnings
```

---

### 99-comprehensive-showcase.yaml - Complete Feature Reference

**What it demonstrates:**
- Full profile metadata configuration
- All 5 plugins (file, http, dns, tcp, command)
- Variable substitution (`vars`)
- Control defaults (severity, owner, tags, timeout, retries)
- Dependencies between controls (`depends_on`)
- Multiple observations per control
- Advanced expect expressions (`in`, `&&`, `||`, comparisons)
- Retry configuration (retries, delay, backoff)
- Execution levels and parallelism (DAG)

**Requirements:** Network access, standard Linux system
**Plugins:** `file`, `http`, `dns`, `tcp`, `command`

**View execution plan:**
```bash
./bin/reglet plan docs/examples/99-comprehensive-showcase.yaml
./bin/reglet plan docs/examples/99-comprehensive-showcase.yaml --details
```

**Run with:**
```bash
./bin/reglet check docs/examples/99-comprehensive-showcase.yaml --trust-plugins
```

**Filter examples:**
```bash
# Only security controls
./bin/reglet plan docs/examples/99-comprehensive-showcase.yaml --tags security

# JSON output
./bin/reglet plan docs/examples/99-comprehensive-showcase.yaml --format json
```

## Running Examples

### Basic usage
```bash
./bin/reglet check examples/01-quickstart.yaml
```

### Filter by tags
```bash
./bin/reglet check examples/02-ssh-hardening.yaml --tags ssh,authentication
```

### Filter by severity
```bash
./bin/reglet check examples/02-ssh-hardening.yaml --severity critical
```

### Exclude controls
```bash
./bin/reglet check examples/02-ssh-hardening.yaml \
  --exclude-control soc2-cc6.1-ssh-x11-forwarding
```

### Different output formats
```bash
# Table (default)
./bin/reglet check examples/01-quickstart.yaml

# JSON
./bin/reglet check examples/01-quickstart.yaml --output json

# YAML
./bin/reglet check examples/01-quickstart.yaml --output yaml

# Save to file
./bin/reglet check examples/01-quickstart.yaml --output json > results.json
```

### Preview execution plan (dry-run)
```bash
# See what would run without executing
./bin/reglet plan examples/99-comprehensive-showcase.yaml

# With detailed control info
./bin/reglet plan examples/99-comprehensive-showcase.yaml --details

# Plan for specific severities
./bin/reglet plan examples/02-ssh-hardening.yaml --severity critical,high
```

## Plugin Summary

| Plugin | Examples | Use Cases |
|--------|----------|----------|
| `file` | 01, 02, 07, 99 | File permissions, content checks, config validation |
| `command` | 06, 99 | Service status, command output validation |
| `http` | 03, 99 | Web endpoints, APIs, status codes, response validation |
| `dns` | 04, 99 | DNS resolution, record validation, propagation checks |
| `tcp` | 05, 99 | Port connectivity, TLS validation, service availability |

## Creating Your Own Profiles

These examples are templates! Copy and modify them:

```bash
# Copy quickstart as a template
cp examples/01-quickstart.yaml my-custom-checks.yaml

# Edit to add your own controls
nano my-custom-checks.yaml

# Run your custom profile
./bin/reglet check my-custom-checks.yaml
```

## Combining Multiple Checks

You can combine checks from different examples:

```yaml
profile:
  name: Complete Infrastructure Audit
  description: File, network, and service checks

plugins:
  - file
  - http
  - dns
  - tcp

controls:
  # From quickstart
  - id: system-security
    observations:
      - plugin: file
        config:
          path: /etc/shadow
        expect: !data.mode.contains("r--r--r--")

  # From web security
  - id: api-available
    observations:
      - plugin: http
        capabilities: [network:outbound:443]
        config:
          url: https://api.example.com/health
        expect: data.status_code == 200

  # From DNS validation
  - id: dns-works
    observations:
      - plugin: dns
        capabilities: [network:outbound:53]
        config:
          hostname: example.com
          record_type: A
        expect: data.record_count > 0

  # From TCP connectivity
  - id: https-port-open
    observations:
      - plugin: tcp
        capabilities: [network:outbound:*]
        config:
          host: example.com
          port: "443"
          tls: true
        expect: |
          data.connected == true &&
          data.tls_version >= "TLS 1.2"
```

## Need Help?

- **Issues:** https://github.com/reglet-dev/reglet/issues
- **Discussions:** https://github.com/reglet-dev/reglet/discussions
