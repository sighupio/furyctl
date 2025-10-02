# Flags Configuration

This document describes how to use the flags configuration feature in furyctl, which allows you to store commonly used command flags directly in your `furyctl.yaml` configuration file.

## Overview

The flags configuration feature helps ensure consistent flag usage across team members and environments by allowing you to define default flag values in your configuration file. This eliminates the need to remember and repeatedly type the same flags.

## Priority System

The flags configuration follows a priority system where values can be overridden:

1. **furyctl.yaml flags** (lowest priority) - Values defined in the `flags` section
2. **Environment variables** (medium priority) - Values set via `FURYCTL_*` environment variables  
3. **Command line flags** (highest priority) - Values passed directly to the command

This means you can set defaults in your configuration file, but still override them when needed.

## Important Limitations

### Config Flags Are Not Supported in YAML

**⚠️ Critical Note**: You cannot specify `config` flags within the `flags` section of your `furyctl.yaml` file. This includes flags like:
- `--config` / `-c` for specifying configuration file paths
- Any flag that points to configuration files

**Why this limitation exists**: This prevents **recursive loading** scenarios where:
1. `furyctl` loads `furyctl.yaml`
2. Finds a `config` flag pointing to another configuration file
3. Attempts to load that file, potentially creating infinite loops or unexpected behavior

**What still works**: 
- CLI config flags work normally: `furyctl apply --config /path/to/config.yaml`
- Environment variables for config: `FURYCTL_CONFIG=/path/to/config.yaml`
- All other flags can be configured in YAML without any issues

**Example of what NOT to do**:
```yaml
# ❌ This will cause a FATAL error
flags:
  apply:
    config: "/path/to/other-config.yaml"  # NOT SUPPORTED
```

**Example of what works perfectly**:
```yaml
# ✅ All other flags work great
flags:
  apply:
    dryRun: true
    timeout: 3600
    distroLocation: "/path/to/distribution"
```

## Configuration Structure

Add a `flags` section to your furyctl.yaml file:

```yaml
apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: my-cluster
spec:
  # ... your cluster configuration

flags:
  global:
    # Global flags that apply to all commands
    debug: true
    disableAnalytics: false
    
  apply:
    # Flags specific to the 'apply' command
    skipDepsValidation: true
    timeout: 7200
    
  delete:
    # Flags specific to the 'delete' command
    dryRun: true
```

## Supported Commands

The following commands support flags configuration:

- `global` - Flags that apply to all commands
- `apply` - Cluster deployment and updates
- `delete` - Cluster deletion
- `create` - Initial cluster configuration creation
- `get` - Information retrieval
- `diff` - Configuration comparison
- `validate` - Configuration validation
- `download` - Dependencies download
- `connect` - VPN connections
- `renew` - Certificate renewal
- `dump` - Template rendering

## Dynamic Values

You can use dynamic value substitution in flags, just like in the main configuration:

```yaml
flags:
  global:
    # Use environment variables
    workdir: "{env://PWD}/workspace"
    outdir: "{env://HOME}/.furyctl/output"
    log: "{env://HOME}/.furyctl/logs/furyctl.log"
    
  apply:
    # Reference files relative to config location
    distroLocation: "{file://./custom-distribution}"
    distroPatches: "{file://./patches}"
    
    # Use environment variables for paths
    binPath: "{env://HOME}/.local/bin"
```

## Examples

### Basic Configuration

```yaml
apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: basic-cluster
spec:
  distributionVersion: v1.31.0
  # ... rest of your cluster configuration

flags:
  # Global flags apply to all commands
  global:
    debug: true
    disableAnalytics: false
    gitProtocol: "https"
    workdir: "/tmp/furyctl-workspace"
    
  # Apply command specific flags
  apply:
    skipDepsValidation: true
    dryRun: false
    timeout: 7200
    vpnAutoConnect: false
    skipVpnConfirmation: false
    force: ["upgrades"]
    
  # Delete command specific flags  
  delete:
    dryRun: true
    autoApprove: false
    
  # Create command specific flags
  create:
    provider: "onpremises"
    version: "v1.31.0"
```

### Dynamic Values

```yaml
apiVersion: kfd.sighup.io/v1alpha2
kind: EKS
metadata:
  name: dynamic-cluster
spec:
  distributionVersion: v1.31.0
  # ... rest of your cluster configuration

flags:
  global:
    # Use environment variables
    workdir: "{env://PWD}/furyctl-workspace"
    outdir: "{env://HOME}/.furyctl/output"
    log: "{env://HOME}/.furyctl/logs/furyctl.log"
    
  apply:
    # Reference files relative to config location
    distroLocation: "{file://./custom-distribution}"
    distroPatches: "{file://./patches}"
    
    # Use environment variables for paths
    binPath: "{env://HOME}/.local/bin"
    
    # Combine static and dynamic values
    upgradePathLocation: "{env://PWD}/upgrade-paths/v1.31.0"
    
  delete:
    # Use environment variable for dry-run control
    dryRun: "{env://FURYCTL_DRY_RUN}"  # Set to "true" or "false"
```

### Team Standardization

```yaml
apiVersion: kfd.sighup.io/v1alpha2
kind: EKS
metadata:
  name: production-cluster
spec:
  distributionVersion: v1.31.0
  # ... rest of your cluster configuration

flags:
  global:
    # Standardize debug output for the team
    debug: false
    
    # Ensure analytics are disabled for compliance
    disableAnalytics: true
    
    # Standardize git protocol for corporate environments
    gitProtocol: "ssh"
    
    # Use consistent workspace location
    workdir: "/workspace/furyctl"
    
  apply:
    # Safety defaults for production environments
    dryRun: false
    
    # Skip dependency validation for faster deployments in CI/CD
    skipDepsValidation: false
    skipDepsDownload: false
    
    # Increase timeout for large clusters
    timeout: 10800  # 3 hours
    podRunningCheckTimeout: 600  # 10 minutes
    
    # VPN settings for corporate network
    vpnAutoConnect: true
    skipVpnConfirmation: true
    
    # Force certain behaviors for consistency
    force: ["pods-running-check", "upgrades"]
    
  delete:
    # Safety first - always dry run by default
    dryRun: true
    
    # Require manual approval for deletions
    autoApprove: false
```

### Upgrade Scenario

```yaml
apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: upgrade-cluster
spec:
  distributionVersion: v1.31.0
  # ... rest of your cluster configuration

flags:
  global:
    # Enable detailed logging during upgrades
    debug: true
    log: "{env://PWD}/upgrade-logs/upgrade-{env://USER}.log"
    
  apply:
    # Upgrade-specific settings
    upgrade: true
    upgradePathLocation: "{file://./upgrade-paths}"
    
    # Skip certain validations during upgrades
    skipDepsValidation: true
    skipNodesUpgrade: false
    
    # Extended timeouts for upgrade operations
    timeout: 14400  # 4 hours
    podRunningCheckTimeout: 900  # 15 minutes
    
    # Force upgrade-related operations
    force: ["upgrades", "migrations"]
    
    # Run specific phases for controlled upgrades
    phase: "distribution"
    
    # Post-upgrade validation phases
    postApplyPhases: ["validation", "smoke-tests"]
    
    # Safety settings
    dryRun: false
    
  # Other commands like validate, download, connect, renew, dump
  # also support flags - see their specific documentation or use --help
```

## Usage

To use flags configuration:

1. Add a `flags` section to your `furyctl.yaml` file
2. Configure the flags you want to set by default
3. Run furyctl commands normally - the flags will be automatically applied

```bash
# The flags from your configuration will be automatically used
furyctl apply

# You can still override flags when needed
furyctl apply --timeout 3600 --dry-run
```

## Best Practices

1. **Start simple** - Begin with basic flags and add more as needed
2. **Use dynamic values** - Leverage environment variables for user-specific paths
3. **Safety first** - Consider setting `dryRun: true` by default for destructive operations
4. **Team consistency** - Use flags configuration to standardize behavior across your team
5. **Document choices** - Add comments explaining why certain flags are set

## Validation

The flags configuration includes built-in validation that will **stop execution** with FATAL errors for:

- **Unsupported flags** for specific commands - Commands will exit immediately with detailed error messages
- **Invalid flag values** (e.g., negative timeouts, invalid protocols)
- **Conflicting flag combinations** (e.g., `upgrade` with `upgradeNode`)
- **Configuration file access issues** (permissions, malformed YAML)

**⚠️ Breaking Change**: Invalid flags now cause FATAL errors and stop execution immediately. This ensures configuration problems are caught early rather than potentially causing issues later in the process.

**Note**: If no `furyctl.yaml` file is present, commands that don't require configuration (like `furyctl create pki`) will work normally without any errors.

## Supported Flags

**Note:** All flag names in the `furyctl.yaml` file use camelCase format (e.g., `disableAnalytics`, `skipDepsValidation`). These are automatically converted to kebab-case internally for CLI compatibility (e.g., `--disable-analytics`, `--skip-deps-validation`).

### Global Flags

- `debug` (bool) - Enable debug output
- `disableAnalytics` (bool) - Disable analytics
- `noTty` (bool) - Disable TTY
- `workdir` (string) - Working directory
- `outdir` (string) - Output directory
- `log` (string) - Log file path
- `gitProtocol` (string) - Git protocol to use ("https" or "ssh")

### Apply Command Flags

- `phase` (string) - Limit execution to specific phase
- `startFrom` (string) - Start execution from specific phase
- `distroLocation` (string) - Distribution location
- `distroPatches` (string) - Distribution patches location
- `binPath` (string) - Binary path
- `skipNodesUpgrade` (bool) - Skip nodes upgrade
- `skipDepsDownload` (bool) - Skip dependencies download
- `skipDepsValidation` (bool) - Skip dependencies validation
- `dryRun` (bool) - Dry run mode
- `vpnAutoConnect` (bool) - Auto connect VPN
- `skipVpnConfirmation` (bool) - Skip VPN confirmation
- `force` (array) - Force options
- `postApplyPhases` (array) - Post apply phases
- `timeout` (int) - Timeout in seconds
- `podRunningCheckTimeout` (int) - Pod running check timeout
- `upgrade` (bool) - Enable upgrade mode
- `upgradePathLocation` (string) - Upgrade path location
- `upgradeNode` (string) - Specific node to upgrade

### Delete Command Flags

- `phase` (string) - Limit execution to specific phase
- `startFrom` (string) - Start execution from specific phase
- `distroLocation` (string) - Distribution location
- `distroPatches` (string) - Distribution patches location
- `binPath` (string) - Binary path
- `dryRun` (bool) - Dry run mode
- `skipVpnConfirmation` (bool) - Skip VPN confirmation
- `autoApprove` (bool) - Auto approve deletion

### Create Command Flags

- `name` (string) - Cluster name
- `version` (string) - Distribution version
- `provider` (string) - Provider type
- `path` (string) - Path where to save PKI files
- `etcd` (bool) - Create PKI only for etcd
- `controlplane` (bool) - Create PKI only for Kubernetes control plane

### Get, Diff, and Tools Command Flags

**Get Command:**
- `binPath` (string) - Binary path
- `distroLocation` (string) - Distribution location
- `skipDepsDownload` (bool) - Skip dependencies download
- `skipDepsValidation` (bool) - Skip dependencies validation

**Diff Command:**
- `phase` (string) - Limit execution to specific phase
- `distroLocation` (string) - Distribution location
- `distroPatches` (string) - Distribution patches location
- `binPath` (string) - Binary path
- `upgradePathLocation` (string) - Upgrade path location

**Validate Command:**
- `distroLocation` (string) - Distribution location
- `distroPatches` (string) - Distribution patches location

**Download Command:**
- `binPath` (string) - Binary path  
- `distroLocation` (string) - Distribution location
- `distroPatches` (string) - Distribution patches location

**Other Commands:**
- `Connect`, `Renew`, `Dump` commands currently have no configurable flags - use `furyctl [command] --help` for details