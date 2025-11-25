# Reglet Plugin Interface (WIT)

This directory contains the WebAssembly Interface Types (WIT) definition for Reglet plugins.

## What is WIT?

WIT (WebAssembly Interface Types) is a language-agnostic interface definition language for WebAssembly. It defines the contract between the Reglet host (written in Go) and plugins (written in any language that compiles to WASM).

## Interface Overview

The `reglet.wit` file defines three main interfaces:

### 1. `types` - Core Data Types

- **config**: Plugin configuration from YAML profiles
- **evidence**: Results and collected data from observations
- **value**: Type-safe values (string, int, float, bool, list)
- **error**: Structured error responses
- **capability**: Permission declarations
- **plugin-info**: Plugin metadata
- **config-schema**: JSON Schema for configuration validation

### 2. `plugin` - Main Plugin Interface

All plugins must implement these three functions:

```wit
describe: func() -> plugin-info
schema: func() -> config-schema
observe: func(config: config) -> result<evidence, error>
```

#### `describe()`
Returns plugin metadata including name, version, description, and required capabilities. Called once during initialization.

#### `schema()`
Returns JSON Schema describing the plugin's configuration format. Used for pre-flight validation before execution.

#### `observe(config)`
Main execution function. Takes configuration, performs the observation, returns evidence or error.

### 3. `reglet-plugin` - Plugin World

Defines what a plugin exports. Currently just the `plugin` interface, but could be extended in the future.

## Using This Interface

### For Plugin Developers

When writing a plugin in your language of choice:

1. Use a WIT binding generator for your language (e.g., `wit-bindgen` for Rust)
2. Implement the three required functions
3. Compile to WASM with the component model

**Example (Rust):**
```rust
wit_bindgen::generate!({
    path: "../wit/reglet.wit",
    world: "reglet-plugin",
});

struct FilePlugin;

impl Plugin for FilePlugin {
    fn describe() -> PluginInfo {
        PluginInfo {
            name: "file".to_string(),
            version: "1.0.0".to_string(),
            description: "File existence and content checks".to_string(),
            capabilities: vec![
                Capability {
                    kind: "fs".to_string(),
                    pattern: "read:/etc/**".to_string(),
                }
            ],
        }
    }

    fn schema() -> ConfigSchema {
        // Return JSON Schema
    }

    fn observe(config: Config) -> Result<Evidence, Error> {
        // Perform observation
    }
}
```

### For Host (Reglet CLI)

The Go host uses `wazero` to load and execute plugins:

1. Load the WASM module
2. Call `describe()` to get metadata and capabilities
3. Validate capabilities against system config
4. Call `schema()` and validate observation configs
5. Call `observe(config)` to execute checks

## Version Compatibility

The interface version is defined in the package declaration:

```wit
package reglet:plugin@1.0.0;
```

Breaking changes require a major version bump. Plugins declare which interface version they implement, and the host checks compatibility at load time.

## Future Extensions

Potential future additions (backward compatible):

- Streaming observations for long-running checks
- Lifecycle hooks (init, cleanup)
- Plugin-to-plugin communication
- Capability negotiation
