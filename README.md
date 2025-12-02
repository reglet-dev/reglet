# Reglet

Reglet is a modular compliance and infrastructure validation engine. It allows engineering teams to define policy as code, execute validation logic in sandboxed WebAssembly environments, and generate standardized audit artifacts.

Designed to bridge the gap between DevOps workflows and rigid compliance frameworks, Reglet treats compliance checks as testable, versioned code rather than manual checklists.

## Core Features

*   **Declarative Policy**: Define validation rules in clear, human-readable YAML profiles.
*   **WASM Extensibility**: All validation logic runs within secure WebAssembly sandboxes (via wazero). This ensures plugin isolation and allows checks to be written in any language that compiles to WASM.
*   **Stateless Architecture**: Runs as a single binary without external database dependencies. Results are strictly a function of the input profile and current system state.
*   **Standardized Output**: Generates machine-readable results (JSON, YAML) designed for integration with the NIST Open Security Controls Assessment Language (OSCAL).
*   **Performance**: Supports parallel execution of controls and observations for fast feedback loops in CI/CD.

## Getting Started

### Prerequisites

*   Go 1.25 or later
*   Make

### Installation

Clone the repository and build the binary:

```bash
git clone https://github.com/whiskeyjimbo/reglet.git
cd reglet
make build
```

The resulting binary will be located at `bin/reglet`.

### Basic Usage

Reglet operates by executing a **Profile**, which defines the controls to validate.

**1. Check a profile:**

```bash
./bin/reglet check test-profile.yaml
```

**2. Output results as JSON:**

```bash
./bin/reglet check test-profile.yaml --format json
```

**3. Save results to a file:**

```bash
./bin/reglet check test-profile.yaml --output report.json --format json
```

## Architecture

Reglet follows an open-core philosophy with a strict focus on security and portability.

*   **Runtime**: The core engine embeds the `wazero` runtime, requiring no CGO or external dependencies.
*   **Plugins**: Functionality is delivered exclusively through WASM modules. Core plugins (File, HTTP, TCP, etc.) are embedded in the binary but run with the same isolation guarantees as third-party plugins.
*   **Capabilities**: Plugins operate under a capability-based security model. They must explicitly request permissions (e.g., `fs:read:/etc`, `network:outbound:443`) which the user controls via configuration.

## Development Status

**Current Status:**

The project is currently in active development. The core execution engine, configuration loader, and CLI foundation are stable.

*   **Completed**: WASM runtime integration, parallel execution engine, file system plugin, structured output formatters, CLI (Cobra/Viper).
*   **In Progress**: Advanced networking plugins (HTTP, DNS), profile inheritance, and expanded OSCAL support.