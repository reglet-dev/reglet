# Contributing to Reglet

Thank you for your interest in contributing to Reglet! We welcome contributions from the community to help make compliance as code more secure and accessible.

## Getting Started

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally.
3.  **Install prerequisites**:
    *   Go 1.25+
    *   Make
    *   `golangci-lint` (for linting)

## Development Workflow

We use a `Makefile` to automate common development tasks:

```bash
# Build the binary
make build

# Run all tests
make test

# Run linter
make lint

# Build and run locally
make dev
```

### Working with Plugins

Reglet uses WASM plugins for all system interactions. When making changes to plugins:

1.  Go to the plugin directory (e.g., `plugins/file`).
2.  Run `make build` to compile the WASM module.
3.  Ensure the updated `.wasm` file is in the correct location for the loader.

## Code Style and Standards

*   **Go Version**: We use Go 1.25+.
*   **Formatting**: run `make fmt` before committing.
*   **Linting**: We check code with `golangci-lint`.
*   **Testing**: New features must include tests. We aim for high test coverage.

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

*   `feat(scope): description` - New feature
*   `fix(scope): description` - Bug fix
*   `docs(scope): description` - Documentation changes
*   `test(scope): description` - Adding missing tests
*   `chore(scope): description` - Maintenance tasks

**Example:** `feat(engine): implement parallel execution for independent controls`

## Pull Request Process

1.  Create a new branch for your feature or fix.
2.  Commit your changes using descriptive, conventional commit messages.
3.  Push your branch to your fork.
4.  Open a Pull Request against the `main` branch.
5.  Ensure all CI checks pass.

## Reporting Issues

If you find a bug or have a feature request, please open an issue on GitHub. Provide as much detail as possible, including:

*   Reglet version
*   Steps to reproduce
*   Expected vs. actual behavior

## License

By contributing to Reglet, you agree that your contributions will be licensed under its Apache-2.0 License.
