# Reglet Examples

These examples demonstrate Reglet's capabilities with real-world checks.

## Quick Start

Try the quickstart example first - it works on any Linux system:

```bash
./bin/reglet check examples/01-quickstart.yaml
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

## Plugin Summary

| Plugin | Examples | Use Cases |
|--------|--------|----------|-----------|
| `file` | 01, 02 | File permissions, content checks, config validation |
| `command` | 02 | Service status, command output validation |
| `http` | 03 | Web endpoints, APIs, status codes, response validation |
| `dns` | 04 | DNS resolution, record validation, propagation checks |
| `tcp` | 05 | Port connectivity, TLS validation, service availability |

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

## Next Steps

- **Read the guide:** [docs/getting-started.md](../docs/getting-started.md)
- **Profile syntax:** [docs/writing-profiles.md](../docs/writing-profiles.md)
- **Plugin reference:** [docs/plugins.md](../docs/plugins.md)

## Need Help?

- **Issues:** https://github.com/whiskeyjimbo/reglet/issues
- **Discussions:** https://github.com/whiskeyjimbo/reglet/discussions
