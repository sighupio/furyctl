# furyctl tools - Tool Integration Management

> [!NOTE]
> The `furyctl tools` command provides multiple output formats for integrating furyctl-downloaded tools with your shell environment. Choose the format that works best with your development setup.

## Overview

The `furyctl tools` command suite solves a common problem in development environments: ensuring that tools downloaded byf furyctl take precedence over system-installed versions or those managed by version managers like mise, asdf, or other tooling.

When you run any `furyctl tools` command, dependencies are **automatically downloaded** to the `.furyctl/bin` directory with specific versions required by your SIGHUP Distribution. 

**Important**: By default, this directory is created in your home directory (`~/.furyctl/bin`), but **best practice is to use `--outdir $PWD`** to create project-specific tooling in your current directory (`./.furyctl/bin`). This provides better project isolation and team collaboration.

These tools won't be in your PATH by default, and version managers may override them with different versions.

The tools command provides three integration strategies:

- **`aliases`**: Traditional bash aliases (simple, but limited by shell precedence)
- **`functions`**: Bash functions with higher precedence (override version managers)
- **`mise`**: Native mise integration via mise.toml (seamless mise workflow)

## Prerequisites and Setup

### furyctl Configuration

Before using the tools commands, ensure you have:

1. A valid `furyctl.yaml` configuration file in your project directory
2. That's it! Dependencies are automatically downloaded when you run any tools command

**Recommended workflow** (project-specific tooling):
```bash
# Use project-specific .furyctl directory (recommended)
furyctl tools aliases --outdir $PWD

# Verify tools are available in your project
ls ./.furyctl/bin/
# Output: kubectl/ terraform/ helm/ ...
```

**Alternative workflow** (global tooling):
```bash
# Use global .furyctl directory (in home folder)
furyctl tools aliases

# Verify tools are available globally
ls ~/.furyctl/bin/
# Output: kubectl/ terraform/ helm/ ...
```

### Understanding the .furyctl/bin Structure

Tools are organized by name and version:

```
.furyctl/bin/
├── kubectl/
│   └── 1.25.8/
│       └── kubectl
├── terraform/
│   └── 1.5.7/
│       └── terraform
└── helm/
    └── 3.10.0/
        └── helm
```

The tools commands automatically discover these tools and their versions from your `kfd.yaml` configuration.

## Shell Precedence and Integration Strategies

Understanding shell precedence is crucial for choosing the right integration method:

1. **Functions** (highest precedence)
2. **Aliases** 
3. **Built-ins**
4. **PATH executables** (including version managers)

### When to Use Each Format

<details>
<summary><strong>Use Aliases When</strong></summary>

- You have a simple shell setup without version managers
- You want lightweight integration
- You don't need to override existing tools
- You're working in environments where functions aren't supported

```bash
# Good for simple setups (project-specific recommended)
eval "$(furyctl tools aliases --outdir $PWD)"
kubectl version  # Uses furyctl's kubectl
```

</details>

<details>
<summary><strong>Use Functions When</strong></summary>

- You have version managers (mise, asdf, etc.) that create aliases
- You need to ensure furyctl tools take precedence
- You want to override system-installed tools
- You need reliable tool version consistency across team members

```bash
# Override mise/asdf/other version managers (project-specific recommended)
eval "$(furyctl tools functions --outdir $PWD)"
kubectl version  # Always uses furyctl's kubectl, regardless of mise
```

</details>

<details>
<summary><strong>Use Mise Integration When</strong></summary>

- You're already using mise for version management
- You want native mise tooling support
- You prefer configuration files over shell evaluation
- You want per-project tool versions with mise's trust system

```bash
# Native mise workflow (project-specific recommended)
furyctl tools mise --outdir $PWD
mise install  # No downloads needed, just uses furyctl paths
kubectl version  # Uses furyctl's kubectl through mise
```

</details>

## Best Practices

### Project-Specific vs Global Tooling

**Recommended: Project-Specific Tooling (`--outdir $PWD`)**
- **Isolation**: Each project gets its own tool versions
- **Team Consistency**: All team members use identical tool versions
- **Version Control**: `.furyctl/` can be gitignored or committed based on team preference
- **No Conflicts**: Different projects can use different SIGHUP Distribution versions

```bash
# Always use project-specific tooling (recommended)
eval "$(furyctl tools functions --outdir $PWD)"
```

**Alternative: Global Tooling (default)**
- **Convenience**: Tools available across all projects
- **Shared Storage**: Single download location saves disk space
- **Potential Issues**: Version conflicts between projects with different requirements

```bash
# Global tooling (use with caution)
eval "$(furyctl tools functions)"
```

### Team Collaboration

For team environments, establish consistent practices:

```bash
# Add to your project's setup documentation
echo 'eval "$(furyctl tools functions --outdir $PWD)"' >> setup.sh

# Or create a team alias
alias fury-tools='eval "$(furyctl tools functions --outdir $PWD)"'
```

## Command Reference

### `furyctl tools aliases`

Generate traditional bash aliases for downloaded tools.

#### Usage

```bash
furyctl tools aliases [flags]
```

#### Examples

```bash
# View generated aliases (project-specific - recommended)
furyctl tools aliases --outdir $PWD

# Output:
# alias kubectl="/path/to/project/.furyctl/bin/kubectl/1.25.8/kubectl"
# alias helm="/path/to/project/.furyctl/bin/helm/3.10.0/helm" 
# alias terraform="/path/to/project/.furyctl/bin/terraform/1.5.7/terraform"

# Set aliases in current session (project-specific)
eval "$(furyctl tools aliases --outdir $PWD)"

# Add to shell profile for project-specific setup
echo 'eval "$(furyctl tools aliases --outdir $PWD)"' >> ~/.bashrc

# Or create project setup script
echo '#!/bin/bash' > setup-tools.sh
echo 'eval "$(furyctl tools aliases --outdir $PWD)"' >> setup-tools.sh
chmod +x setup-tools.sh
```

#### Limitations

- **Version Manager Override**: Aliases can be overridden by mise, asdf, or other version managers
- **Precedence Issues**: May not work if tools are aliased elsewhere
- **Shell Dependency**: Different shells may handle aliases differently

### `furyctl tools functions`

Generate bash functions that override version managers and aliases.

#### Usage

```bash
furyctl tools functions [flags]
```

#### Examples

```bash
# View generated functions (project-specific - recommended)
furyctl tools functions --outdir $PWD

# Output:
# kubectl() { "/path/to/project/.furyctl/bin/kubectl/1.25.8/kubectl" "$@"; }
# helm() { "/path/to/project/.furyctl/bin/helm/3.10.0/helm" "$@"; }
# terraform() { "/path/to/project/.furyctl/bin/terraform/1.5.7/terraform" "$@"; }

# Set functions in current session (overrides mise/asdf, project-specific)
eval "$(furyctl tools functions --outdir $PWD)"

# Add to shell profile for project-specific setup
echo 'eval "$(furyctl tools functions --outdir $PWD)"' >> ~/.bashrc
```

#### Benefits

- **Highest Precedence**: Functions override aliases and version managers
- **Parameter Passing**: Properly forwards all arguments and options
- **Version Manager Override**: Works even with active mise/asdf installations
- **Team Consistency**: Ensures all team members use the same tool versions

### `furyctl tools mise`

Generate or update mise.toml configuration for native mise integration.

#### Usage

```bash
furyctl tools mise [flags]
```

#### Examples

```bash
# Generate or update mise.toml in current directory (project-specific - recommended)
furyctl tools mise --outdir $PWD

# Check the generated configuration
cat mise.toml
# Output:
# [tools]
# kubectl = "path:/path/to/.furyctl/bin/kubectl/1.25.8"
# helm = "path:/path/to/.furyctl/bin/helm/3.10.0"
# terraform = "path:/path/to/.furyctl/bin/terraform/1.5.7"

# Now use mise normally - no downloads needed
mise install
mise list
kubectl version  # Uses furyctl's kubectl
```

#### TOML Configuration Structure

The mise command generates TOML configuration using the `path:` prefix:

```toml
[tools]
kubectl = "path:/home/user/.furyctl/bin/kubectl/1.25.8"
terraform = "path:/home/user/.furyctl/bin/terraform/1.5.7"
helm = "path:/home/user/.furyctl/bin/helm/3.10.0"
```

#### Preserving Existing Configuration

<details>
<summary><strong>Configuration Preservation</strong></summary>

If you have an existing mise.toml file, the command preserves all sections except `[tools]`:

**Before:**
```toml
[tools]
node = "20.0.0"
python = "3.11.0"

[env]
NODE_ENV = "development"
DATABASE_URL = "postgresql://localhost/myapp"

[tasks]
build = "npm run build"
```

**After running `furyctl tools mise`:**
```toml
[tools]
node = "20.0.0"           # Preserved
python = "3.11.0"         # Preserved
kubectl = "path:/home/user/.furyctl/bin/kubectl/1.25.8"  # Added
terraform = "path:/home/user/.furyctl/bin/terraform/1.5.7"  # Added

[env]                     # Fully preserved
NODE_ENV = "development"
DATABASE_URL = "postgresql://localhost/myapp"

[tasks]                   # Fully preserved
build = "npm run build"
```

</details>

#### Reverting mise Integration

The `--revert` flag allows you to remove furyctl-managed tools from your mise.toml while preserving all other configuration:

```bash
# Remove furyctl tools from mise.toml (project-specific - recommended)
furyctl tools mise --revert --outdir $PWD

# Skip confirmation prompt
furyctl tools mise --revert --force --outdir $PWD
```

**Important revert behavior:**
- **Only removes furyctl-discoverable tools**: Tools that furyctl would currently add to mise.toml
- **Preserves non-furyctl tools**: Any tools not managed by furyctl remain untouched
- **Preserves all other sections**: `[env]`, `[tasks]`, `[alias]` sections remain intact
- **Interactive confirmation**: Shows exactly which tools will be removed (unless `--force` is used)

<details>
<summary><strong>Revert Example</strong></summary>

**Before revert (mixed tools in mise.toml):**
```toml
[tools]
node = "20.0.0"                    # Not managed by furyctl
python = "3.11.0"                  # Not managed by furyctl
kubectl = "path:/path/to/.furyctl/bin/kubectl/1.25.8"   # Managed by furyctl
terraform = "path:/path/to/.furyctl/bin/terraform/1.5.7" # Managed by furyctl
custom-tool = "path:/opt/custom"   # Not managed by furyctl

[env]
NODE_ENV = "development"
```

**Command:**
```bash
furyctl tools mise --revert --outdir $PWD

# Interactive output:
# The following furyctl-managed tools will be removed from mise.toml:
#   kubectl
#   terraform
# 
# Are you sure you want to continue? Only 'yes' will be accepted to confirm.
# yes
```

**After revert:**
```toml
[tools]
node = "20.0.0"                    # Preserved
python = "3.11.0"                  # Preserved  
custom-tool = "path:/opt/custom"   # Preserved

[env]                              # Fully preserved
NODE_ENV = "development"
```

</details>

<details>
<summary><strong>Revert Use Cases</strong></summary>

**When to use `--revert`:**
- **Switching to functions**: Move from mise integration to function-based integration
- **Clean slate**: Remove furyctl tools before updating to a new distribution version
- **Troubleshooting**: Reset mise integration when experiencing issues
- **Project handoff**: Clean up mise.toml before transferring project to different tooling

**Examples:**
```bash
# Switch from mise to functions integration
furyctl tools mise --revert --force --outdir $PWD
eval "$(furyctl tools functions --outdir $PWD)"

# Clean up before updating furyctl config
furyctl tools mise --revert --outdir $PWD
# ... update furyctl.yaml to new distribution version ...
furyctl tools mise --outdir $PWD  # Re-add with new versions

# Remove all furyctl tools but keep custom mise config
furyctl tools mise --revert --force --outdir $PWD
# Your node, python, and other non-furyctl tools remain in mise.toml
```

</details>

### Air-Gapped Environment Usage

<details>
<summary><strong>Pre-downloaded Tools Setup</strong></summary>

In air-gapped environments, tools must be pre-downloaded:

```bash
# On internet-connected machine
furyctl download dependencies --outdir $PWD
tar czf furyctl-tools.tar.gz .furyctl/

# Transfer to air-gapped environment
scp furyctl-tools.tar.gz airgapped-server:

# On air-gapped machine
tar xzf furyctl-tools.tar.gz
eval "$(furyctl tools functions --skip-deps-download --outdir $PWD)"

# Verify tools work
kubectl version --client
```

</details>

## Common Flags

All tools commands support these flags:

- `--config, -c`: Path to furyctl configuration file (default: "furyctl.yaml")
- `--bin-path, -b`: Path to tools directory (default: ".furyctl/bin")
- `--distro-location`: Override distribution location for air-gapped setups
- `--skip-deps-download`: Skip downloading dependencies, use existing tools

### mise-specific Flags

The `furyctl tools mise` command supports additional flags:

- `--revert`: Remove furyctl-managed tools from mise.toml instead of adding them
- `--force`: Skip confirmation prompt when reverting tools

### Examples with Flags

```bash
# Use custom configuration file (project-specific recommended)
furyctl tools aliases --config /path/to/custom-furyctl.yaml --outdir $PWD

# Use custom bin directory (project-specific recommended)
furyctl tools functions --bin-path /opt/furyctl/bin --outdir $PWD

# Air-gapped usage (project-specific recommended)
furyctl tools mise --skip-deps-download --distro-location file:///opt/fury-distribution --outdir $PWD

# Combine flags (project-specific recommended)
furyctl tools aliases -c dev-furyctl.yaml -b /tmp/furyctl-bin --skip-deps-download --outdir $PWD
```

## Troubleshooting Guide

### Common Issues and Solutions

<details>
<summary><strong>Issue: Commands not found after setting aliases/functions</strong></summary>

**Symptoms**: `kubectl: command not found` after running `eval "$(furyctl tools aliases)"`

**Diagnosis**:
```bash
# Check if tools were discovered
furyctl tools aliases
# Should show aliases, not error messages

# Check if dependencies were downloaded
ls .furyctl/bin/
# Should show tool directories

# Check current aliases/functions
alias | grep kubectl
type kubectl
```

**Solutions**:
```bash
# Dependencies are downloaded automatically when running tools commands
eval "$(furyctl tools aliases --outdir $PWD)"

# For persistent issues, use absolute paths
which furyctl
/full/path/to/furyctl tools aliases --outdir $PWD
```

</details>

<details>
<summary><strong>Issue: Version managers override furyctl tools</strong></summary>

**Symptoms**: `kubectl version` shows wrong version despite setting aliases

**Diagnosis**:
```bash
# Check what's being used
type kubectl
which kubectl

# Check version manager status
mise list        # if using mise
asdf list        # if using asdf

# Check shell precedence
alias kubectl   # should show alias
```

**Solutions**:
```bash
# Use functions instead of aliases (higher precedence, project-specific recommended)
eval "$(furyctl tools functions --outdir $PWD)"

# Or disable version manager for specific tools
mise uninstall kubectl  # mise
asdf uninstall kubectl  # asdf

# Or use mise integration (project-specific recommended)
furyctl tools mise --outdir $PWD
mise install
```

</details>

<details>
<summary><strong>Issue: Mise doesn't recognize furyctl tools</strong></summary>

**Symptoms**: `mise install` doesn't find tools after `furyctl tools mise`

**Diagnosis**:
```bash
# Check mise.toml content
cat mise.toml

# Check mise configuration
mise config

# Check if paths are valid
ls -la /path/to/.furyctl/bin/kubectl/1.25.8/kubectl
```

**Solutions**:
```bash
# Regenerate mise.toml (project-specific recommended)
rm mise.toml
furyctl tools mise --outdir $PWD

# Check mise trust settings
mise trust

# Verify path format
# Should be: kubectl = "path:/absolute/path/to/tool/directory"
```

</details>

<details>
<summary><strong>Issue: No tools found error</strong></summary>

**Symptoms**: `no tools found in .furyctl/bin. Run 'furyctl download dependencies' first`

**Diagnosis**:
```bash
# Check if .furyctl/bin exists and has content
ls -la .furyctl/bin/

# Check if furyctl.yaml is valid
furyctl validate

# Check current directory
pwd
ls furyctl.yaml
```

**Solutions**:
```bash
# Dependencies are downloaded automatically when running tools commands
furyctl tools aliases --outdir $PWD

# If using custom paths
furyctl tools aliases --bin-path /custom/path --outdir $PWD

# For air-gapped environments
furyctl tools aliases --skip-deps-download --outdir $PWD
```

</details>

<details>
<summary><strong>Issue: Revert command not removing expected tools</strong></summary>

**Symptoms**: `furyctl tools mise --revert` says "No furyctl-managed tools found" but you see furyctl tools in mise.toml

**Diagnosis**:
```bash
# Check what tools furyctl currently discovers
furyctl tools mise --outdir $PWD
# Compare with what's in mise.toml
cat mise.toml

# Check if you're in the right directory
pwd
ls furyctl.yaml

# Check if the tool paths match what furyctl expects
ls -la .furyctl/bin/
```

**Solutions**:
```bash
# Make sure you're using the same --outdir and --bin-path as when you added tools
furyctl tools mise --revert --outdir $PWD

# If tools were added with custom bin-path, use the same path
furyctl tools mise --revert --bin-path /custom/path --outdir $PWD

# For manual cleanup, edit mise.toml directly
nano mise.toml
# Remove lines with "path:" pointing to .furyctl directories
```

</details>

<details>
<summary><strong>Issue: Accidental tool removal during revert</strong></summary>

**Symptoms**: Important non-furyctl tools were removed from mise.toml

**Solutions**:
```bash
# Revert only removes tools that furyctl would currently discover
# If a tool was removed, it means furyctl thinks it should manage it

# Check what happened
furyctl tools mise --outdir $PWD
# This shows what furyctl thinks it should manage

# Restore from backup if available
cp mise.toml.backup mise.toml

# Or manually re-add the tool to mise.toml
echo 'my-tool = "1.2.3"' >> mise.toml

# Use --force flag carefully to skip confirmations
furyctl tools mise --revert --force --outdir $PWD  # Use with caution
```

</details>

### Debug Mode

For troubleshooting, use debug mode to see what's happening:

```bash
# Enable debug logging (project-specific recommended)
furyctl tools aliases --debug --outdir $PWD

# Or check logs
furyctl tools functions --debug --outdir $PWD 2>&1 | grep -i error
```


> [!TIP]
> **Best Practice**: Use `--outdir $PWD` for project-specific tooling to ensure team consistency and avoid version conflicts between projects. Dependencies are automatically downloaded when you run any tools command.