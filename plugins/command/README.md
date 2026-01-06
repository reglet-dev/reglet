# Command Plugin

Execute commands and validate output.

## Configuration

### Schema

```yaml
controls:
  # Option 1: Shell command (via /bin/sh -c)
  - id: CMD-001
    plugin: command
    config:
      run: "systemctl is-active sshd"
      timeout: 30                   # Optional, default: 30 seconds

  # Option 2: Direct execution (safer, recommended)
  - id: CMD-002
    plugin: command
    config:
      command: "/usr/bin/systemctl"
      args: ["is-active", "sshd"]
      dir: "/"                      # Optional: working directory
      env: ["MY_VAR=value"]         # Optional: environment variables
      timeout: 30
```

### Required Fields

One of the following (mutually exclusive):

- `run`: Command string to execute via shell (`/bin/sh -c "..."`).
- `command`: Executable path for direct execution.

### Optional Fields

- `args`: Arguments for direct execution (with `command`).
- `dir`: Working directory.
- `env`: Environment variables as `KEY=VALUE` strings.
- `timeout`: Execution timeout in seconds (default: 30).

## Security Warning

⚠️ **Shell Execution**: Using `run` executes commands via `/bin/sh` which can be dangerous:
- Requires explicit `exec:/bin/sh` capability grant.
- Vulnerable to command injection if input is untrusted.
- For untrusted input, use `command` mode with explicit args instead.

## Capabilities

- **exec**: `**`

Example grant in system config:

```yaml
plugins:
  reglet/command@1.0:
    capabilities:
      - exec:/usr/bin/systemctl
      - exec:/bin/sh           # Required for 'run' mode
```

## Evidence Data

### Success (Exit Code 0)

```json
{
  "status": true,
  "data": {
    "stdout": "active",
    "stderr": "",
    "exit_code": 0,
    "duration_ms": 45,
    "is_timeout": false,
    "exec_mode": "shell",
    "shell_command": "systemctl is-active sshd"
  }
}
```

### Failure (Non-Zero Exit)

```json
{
  "status": false,
  "data": {
    "stdout": "inactive",
    "stderr": "",
    "exit_code": 3,
    "duration_ms": 42,
    "is_timeout": false,
    "exec_mode": "direct",
    "command_path": "/usr/bin/systemctl",
    "command_args": ["is-active", "sshd"]
  }
}
```

### Timeout

```json
{
  "status": false,
  "data": {
    "stdout": "",
    "stderr": "",
    "exit_code": -1,
    "duration_ms": 30000,
    "is_timeout": true
  }
}
```

## Development

### Building

```bash
make -C plugins/command build
```

### Testing

```bash
make -C plugins/command test
```

## Platform Requirements

- Reglet Host v0.2.0+
- WASM Runtime with `wasi_snapshot_preview1` support
