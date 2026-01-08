# Process Plugin

Check running processes on Linux systems by reading from `/proc`.

## Platform

**Linux-only** - This plugin reads from the `/proc` filesystem which is Linux-specific.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No* | Process name to match (from /proc/PID/comm) |
| `pid` | integer | No* | Specific process ID to check |
| `pattern` | string | No* | Regex pattern to match against cmdline |

*At least one of `name`, `pid`, or `pattern` is required.

## Evidence Output

The plugin returns a summary for clean table output:

| Field | Type | Description |
|-------|------|-------------|
| `found` | boolean | Whether any matching processes were found |
| `count` | integer | Number of matching processes |

> **Note**: This plugin returns summary data only (`found`, `count`). 
> For detailed process information during debugging, use `--format json` output.

## Examples

### Check if a service is running

```yaml
- id: nginx-running
  name: Nginx is running
  observations:
    - plugin: process
      config:
        name: nginx
      expect:
        - "data.found == true"
        - "data.count >= 1"
```

### Check a specific PID

```yaml
- id: init-process
  name: Init process exists
  observations:
    - plugin: process
      config:
        pid: 1
      expect:
        - "data.found == true"
```

### Find processes by command pattern

```yaml
- id: java-apps
  name: Java applications running
  observations:
    - plugin: process
      config:
        pattern: "java.*-jar.*"
      expect:
        - "data.count > 0"
```

### Check no unwanted processes

```yaml
- id: no-telnet
  name: Telnet daemon not running
  observations:
    - plugin: process
      config:
        name: telnetd
      expect:
        - "data.found == false"
```

## Capabilities

- `fs:read:/proc/**` - Read access to process information
