# Development Guide

This guide covers development setup, building from source, and contributing to furyctl.

## Prerequisites

- **mise** for tool and task management - https://mise.jdx.dev/getting-started.html
- **git** for version control

> **Note**: Go, golangci-lint, and all other development tools are automatically installed by mise. You do NOT need to install Go manually.

## Quick Start

### Setting up with mise (Recommended)

1. **Clone the repository**:
   ```bash
   git clone git@github.com:sighupio/furyctl.git
   cd furyctl
   ```

2. **Install all development tools**:
   ```bash
   mise install      # Installs Go 1.23.2, golangci-lint, goreleaser, awscli, and all Go-based tools
   ```

3. **Setup Go module dependencies**:
   ```bash
   mise run setup    # Downloads and tidies Go module dependencies
   ```

4. **Build and test**:
   ```bash
   mise run build       # Build with goreleaser
   mise run test-unit   # Run unit tests  
   mise run lint        # Run linting
   ```

5. **Find built binary**:
   The built binary will be in the `dist/` directory after running `mise run build`.

> **What gets installed**: `mise install` automatically installs Go 1.23.2, golangci-lint 1.59.1, goreleaser, awscli, addlicense, gofumpt, gci, goimports, formattag, ginkgo, and go-cover-treemap - everything you need for development.

## Development Workflow

### Using mise (Recommended)

The project uses **mise** for task and tool management. Here are the essential commands:

#### Setup and Tools
- `mise install`: Installs ALL development tools automatically:
  - **Native tools**: Go 1.23.2, golangci-lint 1.59.1, goreleaser, awscli 2.15.17
  - **UBI tools**: addlicense v1.1.1, gofumpt v0.6.0  
  - **Go tools**: gci v0.13.4, goimports v0.22.0, formattag v0.0.9, ginkgo v2.19.0, go-cover-treemap v1.4.2
- `mise run setup`: Downloads and tidies Go module dependencies (`go mod download` + `go mod tidy`).

#### Code Formatting and Quality  
- `mise run format-go`: Runs complete Go formatting pipeline (fmt → fumpt → imports → gci → formattag).
- `mise run license-add`: Adds license headers to newly added files.
- `mise run lint`: Runs Go linting to check for style issues or common errors.

#### Testing
- `mise run test-unit`: Runs unit tests.
- `mise run test-integration`: Runs integration tests.
- `mise run test-e2e`: Runs e2e tests.
- `mise run test-most`: Runs most tests except expensive AWS-based ones.
- `mise run test-expensive`: **WARNING:** Runs expensive tests that create AWS clusters.

#### Building and Release
- `mise run build`: Builds the project with goreleaser.
- `mise run release`: Release with goreleaser (requires proper setup).
- `mise run clean`: Cleans build artifacts.

#### Development Utilities  
- `mise run env`: Shows environment variables for development.
- `mise run mod-download`: Downloads go modules.
- `mise run mod-tidy`: Tidies go modules.
- `mise run mod-upgrade`: Upgrades all modules and tidies.

## Testing

### Test Classes

There are four kinds of tests in furyctl:

- **unit**: Tests that exercise a single component or function in isolation. Tests using local files and dirs are permitted here.
- **integration**: Tests that require external services, such as GitHub. Tests using only local files and dirs should not be marked as integration.
- **e2e**: Tests that exercise furyctl binary, invoking it as a CLI tool and checking its output.
- **expensive**: E2E tests that incur monetary cost, like running an EKS instance on AWS.

Each test class covers specific use cases depending on speed, cost, and dependencies. Anything that uses I/O should be marked as integration, with the exception of local files and folders - any test that uses only the local filesystem can be marked as 'unit' for convenience.

### Running Tests

```bash
# Run different test suites
mise run test-unit           # Fast unit tests
mise run test-integration    # Integration tests (require external services)
mise run test-e2e           # End-to-end tests with dry-run
mise run test-most          # All tests except expensive ones
mise run test-expensive     # ⚠️  WARNING: Creates real AWS resources

# View test coverage
mise run test-most          # Generates coverage.out
mise run show-coverage      # Opens coverage report in browser
```

## Code Standards

### Go Coding Standards
- Follow typical Go coding conventions
- Prefer structs with methods over standalone functions  
- Use lowercase for private variables and methods
- Follow Go's naming and formatting conventions

### Pre-Commit Requirements

**MANDATORY**: Before committing any code changes:

1. **Format code**: Run `mise run format-go` to fix all formatting issues automatically
2. **Lint code**: Run `mise run lint` to verify zero linting violations  
3. **Test changes**: Run `mise run test-unit` (or appropriate test suite)
4. **Add specific files**: Use `git add path/to/file` - **NEVER** use `git add .` or `git add -A`

### Linting Guidelines

The project follows strict linting rules. The formatting pipeline handles most issues automatically:

```bash
# The recommended workflow
mise run format-go && mise run lint
```

This runs the complete formatting chain:
- `gofmt` - Standard Go formatting
- `gofumpt` - Stricter formatting rules  
- `goimports` - Import organization
- `gci` - Import grouping and sorting
- `formattag` - Struct tag formatting

The linter configuration has been carefully tuned - **do not modify linter rules** without discussion.

## Release Process

The release process is documented in the [MAINTENANCE.md](https://github.com/sighupio/fury-distribution/blob/main/MAINTENANCE.md#furyctl) file. 

For releases not tied to `fury-distribution`, create a tag and release it. For releases dependent on new distribution versions, the process may be more complex and require updating `fury-distribution` first.

## Project Structure

- **`cmd/`**: Main commands created with Cobra library (apply, delete, etc.)
- **`configs/`**: Patch configurations and upgrade paths for the distribution
  - `patches/`: Version-specific patches applied to fury-distribution
  - `provisioners/`: Terraform templates for EKS provider PreFlight phase  
  - `upgrades/`: Upgrade hooks organized by version (e.g., `1.29.5-1.30.0`)
- **`docs/`**: Project documentation, changelogs, and guides
- **`internal/`**: Private code not meant for external use
- **`pkg/`**: Public APIs meant to be used by other packages/projects
- **`test/`**: Test data, configurations, and assets

## Contributing

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes following the code standards above
4. Run the pre-commit checklist (format, lint, test)
5. Create a pull request with a clear description of changes

### Before Every Commit
- [ ] Run `mise run format-go` to format code automatically
- [ ] Run `mise run lint` to verify zero linting violations  
- [ ] Run appropriate test suite (`mise run test-unit` minimum)
- [ ] Add only specific files with `git add path/to/file`
- [ ] **Never** use `git add .` or `git add -A`

## Getting Help

- Open issues for bugs or feature requests on [GitHub Issues](https://github.com/sighupio/furyctl/issues)
- For distribution-related issues, use the [distribution repository](https://github.com/sighupio/distribution) instead
- Check the [FAQ](FAQ.md) for common questions and technical explanations