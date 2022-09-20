<!-- markdownlint-disable MD033 -->
<h1 align="center">
  <img src="docs/assets/furyctl-logo.png" width="200px"/><br/>
  Furyctl
</h1>

<p align="center">The multi-purpose command line tool for the Kubernetes Fury Distribution.</p>
<!-- markdownlint-eable MD033 -->

[![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg)](https://ci.sighup.io/sighupio/furyctl)
![Release](https://img.shields.io/github/v/release/sighupio/furyctl?label=Furyctl)
![Slack](https://img.shields.io/badge/slack-@kubernetes/fury-yellow.svg?logo=slack)
![License](https://img.shields.io/github/license/sighupio/furyctl)

<!-- <KFD-DOCS> -->

Furyctl is a simple CLI tool to:

- download and manage the Kubernetes Fury Distribution (KFD) modules
- create and manage Fury clusters on AWS

<br/>

![Furyctl usage](docs/assets/furyctl.gif)

## Installation

### Installation from binaries

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

To download the latest release, run:

```bash
wget -q "https://github.com/sighupio/furyctl/releases/download/v0.9.0/furyctl-$(uname -s)-amd64" -O /tmp/furyctl
chmod +x /tmp/furyctl
sudo mv /tmp/furyctl /usr/local/bin/furyctl
```

Alternatively, you can install `furyctl` using a brew tap or via an asdf plugin.

> ⚠️ M1 users: please download `darwin/amd64` binaries instead of using homebrew or asdf. Even though furyctl can be build for `arm64`, some of its dependendecies are not available yet for this architecture.

### Installation with [Homebrew](https://brew.sh/)

```bash
brew tap sighupio/furyctl
brew install furyctl
```

### Installation with [asdf](https://github.com/asdf-vm/asdf)

Add furyctl asdf plugin:

```bash
asdf plugin add furyctl
```

Check that everything is working correctly with `furyctl version`:

```bash
furyctl version
INFO[0000] Furyctl version 0.9.0
```

> 💡 **TIP**
>
> Enable autocompletion for `furyctl` CLI on your shell (currently autocompletion is provided for `bash`, `zsh`, `fish`).
> To see the instruction on how to enable it, run `furyctl completion -h`

## Usage

See the available commands with `furyctl --help`:

```bash
furyctl --help

A command-line tool to manage cluster deployment with Kubernetes

Usage:
  furyctl [command]

Available Commands:
  bootstrap   Creates the required infrastructure to deploy a battle-tested Kubernetes cluster, mostly network components
  cluster     Creates a battle-tested Kubernetes cluster
  completion  Generate completion script
  help        Help about any command
  init        Initialize the minimum distribution configuration
  vendor      Download dependencies specified in Furyfile.yml
  version     Prints the client version information
```

## Download and manage KFD modules

`furyctl` can be used as a package manager for the KFD.
It provides a simple way to download all the desired modules of the KFD by reading a single `Furyfile`.

The process requires the following steps:

1. Write a `Furyfile`
2. Run `furyctl vendor` to download all the modules

### 1. Write a Furyfile

A `Furyfile` is a simple YAML formatted file that lists which modules (and versions) of the KFD you want to download.

An example `Furyfile` is the following:

```yaml
# Here you can specify which versions of the modules to use
versions:
  networking: v1.7.0
  monitoring: v1.13.0
  logging: v1.9.1
  ingress: v1.11.2
  dr: v1.8.0
  opa: v1.5.0

# The bases are a sets of Kustomize bases to deploy Kubernetes components
bases:
  - name: networking/
  - name: monitoring/
  - name: logging/
  - name: ingress/
  - name: dr/
  - name: opa/
```

Each module is composed of a set of packages. In the previous `Furyfile`, we downloaded all packages of each module. You can cherry-pick single packages using the `module/package` syntax.

A more complete `Furyfile` would be:

```yaml
# Here you can specify which versions of the modules to use
versions:
  networking: v1.7.0
  monitoring: v1.13.0
  logging: v1.9.1
  ingress: v1.11.2
  dr: v1.8.0
  opa: v1.5.0

# The bases are a sets of Kustomize bases to deploy Kubernetes components
bases:
  - name: networking/calico
  - name: monitoring/prometheus-operator
  - name: monitoring/prometheus-operated
  - name: monitoring/grafana
  - name: monitoring/goldpinger
  - name: monitoring/configs
  - name: monitoring/kubeadm-sm
  - name: monitoring/kube-proxy-metrics
  - name: monitoring/kube-state-metrics
  - name: monitoring/node-exporter
  - name: monitoring/metrics-server
  - name: monitoring/alertmanager-operated
  - name: logging/elasticsearch-single
  - name: logging/cerebro
  - name: logging/curator
  - name: logging/fluentd
  - name: logging/kibana
  - name: ingress/cert-manager
  - name: ingress/nginx
  - name: ingress/forecastle
  - name: dr/velero
  - name: opa/gatekeeper
```

You can find out what packages are inside each module by referring to each module's documentation.

### 2. Download the modules

Run `furyctl vendor` (within the same directory where your `Furyfile` is located) to download the modules.

`furyctl` will download all the packages in a `vendor/` directory.

> 💡 **TIP**
>
> Use the `-H` flag in the `furyctl vendor` command to download using HTTP(S) instead of the default SSH. This is useful if you are in an environment that restricts the SSH traffic.

## Cluster creation

The Cluster creation feature is available via two commands:

- `furyctl bootstrap`: creates the required networking infrastructure
- `furyctl cluster`: creates a Fury cluster.

Both commands provide the following subcommands:

- `furyctl {bootstrap,cluster} template --provisioner {provisioner_name}`: Creates a `yml` configuration file with some default options making easy replacing these with the right values.
- `furyctl {bootstrap,cluster} init`: Initializes the project that deploys the infrastructure.
- `furyctl {bootstrap,cluster} apply`: Actually creates or updates the infrastructure.
- `furyctl {bootstrap,cluster} destroy`: Destroys the infrastructure.

The subcommands accept the following options:

```bash
-c, --config string:   Configuration file path
-t, --token string:    GitHub token to access enterprise repositories. Contact sales@sighup.io
-w, --workdir string:  Working directory with all project files
```

> 💡 **TIP**
>
> You can use the `--dry-run` flag simulate the execution of a command

### Configuration file

The cluster creation feature uses a different configuration file than the `Furyfile.yml`.
While the `Furyfile.yml` file is used by the package-manager features, the cluster creation feature uses a separated `cluster.yml` file:

```yaml
kind:           # Cluster or Bootstrap
metadata:
  name:         # Name of the deployment. It can be used by the provisioners as a unique identifier.
executor:       # This is an optional attribute. It defines the terraform executor to use along with the backend configuration
  state:        # Optional attribute. It configures the backend configuration file.
    backend:    # Optional attribute. It configures the backend to use. Default to local
    config:     # Optional attribute. It configures the configuration of the selected backend configuration. It accepts multiple key values.
      # bucket: "my-bucket"         # Example
      # key: "terraform.tfvars"     # Example
      # region: "eu-home-1"         # Example
provisioner:    # Defines what provisioner to use.
spec: {}        # Input variables of the provisioner. Read each provisioner definition to understand what are the valid values.
```

### Deploy a cluster from zero

The following workflow describes a complete setup of a cluster from scratch.
The bootstrap command will create the underlay requirements to deploy a Kubernetes cluster. Most of these
components are network-related stuff.

Once the bootstrap process is up to date, the cluster command can be triggered using outputs from the
`bootstrap apply` command.

```bash
+--------------------------+   +--------------------------+   +--------------------------+   +--------------------------+
| furyctl bootstrap init   +-->+ furyctl bootstrap apply  +-->+ furyctl cluster init     +-->+ furyctl cluster apply    |
+--------------------------+   +--------------------------+   +--------------------------+   +--------------------------+
```

### Deploy a cluster from an already existing infrastructure

The following workflow describes a setup of a cluster using an already existing underlay infrastructure.

```bash
+--------------------------+   +--------------------------+
+ furyctl cluster init     +-->+ furyctl cluster apply    |
+--------------------------+   +--------------------------+
```

### Installers

To deploy all the infrastructure components `furyctl` uses *installers*.

> You can use an environment variable to avoid passing the token via console: `FURYCTL_TOKEN`.

Contact [sales@sighup.io](mailto:sales@sighup.io) to get more details about this feature.

#### Bootstrap

The available `bootstrap` provisioners are:

| Provisioner | Description                                                                                                                                            |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `aws`       | It creates a VPC with all the requirements to deploy a Kubernetes Cluster. It also includes a VPN instance easily manageable by using `furyagent`.     |

#### Clusters

The available `cluster` provisioners are:

| Provisioner | Description                                                                                                                              |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `eks`       | Creates an EKS cluster on an already existing VPC. It uses the [fury-eks-installer](https://github.com/sighupio/fury-eks-installer)      |

<!-- </KFD-DOCS> -->
<!-- <FOOTER> -->

## Contributing

Before contributing, please read first the [Contributing Guidelines](docs/CONTRIBUTING.md).

### Reporting Issues

In case you experience any problems, please [open a new issue](https://github.com/sighupio/furyctl/issues/new/choose).

## License

This module is open-source and it's released under the following [LICENSE](LICENSE)

<!-- </FOOTER> -->
