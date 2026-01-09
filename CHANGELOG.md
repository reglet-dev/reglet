# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0-alpha] - 2026-01-09

### Added

- **WASM Plugin Runtime** - Sandboxed execution using wazero (pure Go, CGO-free)
- **6 Built-in Plugins** - file, command, http, dns, tcp, smtp
- **Capability-Based Security** - Fine-grained permission system extracted from profiles
- **Configurable Security Levels** - strict, standard, permissive modes
- **Automatic Secret Redaction** - Powered by gitleaks pattern detection
- **Multiple Output Formats** - Table, JSON, YAML, JUnit, SARIF
- **Parallel Execution** - Independent controls run concurrently with DAG-based scheduling
- **Profile System** - OSCAL-aligned declarative YAML configuration
- **Tag & Severity Filtering** - Filter controls by tags or severity levels
- **Environment Variable Injection** - Pass variables into profiles
- **Plugin CLI** - `reglet create plugin` command for scaffolding new plugins
- **Go SDK** - Simplified plugin development with typed errors and context propagation

### Distribution

- Multi-platform binaries (Linux/macOS/Windows, amd64/arm64)
- Docker images on GitHub Container Registry (multi-arch)
- Homebrew tap (`brew install whiskeyjimbo/tap/reglet`)
- Install script with platform auto-detection

### Security

- Path traversal protection in plugin loading
- SSRF protection for HTTP plugin
- Memory limits for WASM execution
- Sandboxed file access via `os.OpenRoot`
- No global mutable state

### Fixed

- Thread-safety issues in WASM runtime with per-call instance creation
- Memory leaks with defer-based deallocation
- Race conditions in parallel execution
- Deterministic output ordering for controls
- IPv4-mapped IPv6 address normalization

### Known Issues

- Windows builds are experimental (not fully tested)
- Profile inheritance not yet implemented
- Plugin registry is RFC-only (manual plugin management)

---

## [0.1.0-alpha] - 2025-12-01

### Added

- Initial proof of concept
- Basic file validation plugin
- YAML profile parsing
- Table output format

[Unreleased]: https://github.com/whiskeyjimbo/reglet/compare/v0.2.0-alpha...HEAD
[0.2.0-alpha]: https://github.com/whiskeyjimbo/reglet/compare/v0.1.0-alpha...v0.2.0-alpha
[0.1.0-alpha]: https://github.com/whiskeyjimbo/reglet/releases/tag/v0.1.0-alpha
