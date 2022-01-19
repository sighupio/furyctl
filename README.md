<h1 align="center">
  <img src="docs/assets/furyctl-logo.png" width="200px"/><br/>
  Furyctl
</h1>

<p align="center">The multi-purpose command line tool for the Kubernetes Fury Distribution.</p>

[![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg)](https://ci.sighup.io/sighupio/furyctl)
![Release](https://img.shields.io/github/v/release/sighupio/furyctl?label=Furyctl)
![Slack](https://img.shields.io/badge/slack-@kubernetes/fury-yellow.svg?logo=slack)
![License](https://img.shields.io/github/license/sighupio/furyctl)

# Furyctl

Furyctl is a simple CLI tool to:

- download and manage the Kubernetes Fury Distribution (KFD) modules
- create and manage Fury clusters on AWS, GCP and vSphere

<br/>

![Furyctl usage](docs/assets/furyctl.gif)


## Installation

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

To download the latest release, run:

```bash
wget -q https://github.com/sighupio/furyctl/releases/download/v0.6.1/furyctl-$(uname -s)-amd64 -O /tmp/furyctl
chmod +x /tmp/furyctl
sudo mv /tmp/furyctl /usr/local/bin/furyctl
```

Alternatively, [Homebrew](https://brew.sh/) users can use `brew` to install `furyctl`:

```bash
brew tap sighupio/furyctl
brew install furyctl
```

Check that everything is working correctly with `furyctl version`:

```bash
➜ furyctl version
INFO[0000] Furyctl version 0.6.1                        
INFO[0000] built 2021-09-20T15:36:15Z from commit 012d862edc6b452752a8955fc624f6064566a6cb 
```

> 💡 **TIP**
>
> Enable autocompletion for `furyctl` cli on your shell (currently autocompletion is supported for `bash`, `zsh`, `fish`).
> To see the instruction to enable it, run `furyctl completion -h`

## Usage

See the available commands with `furyctl --help`:

```bash
$ furyctl --help

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

## Download and manage the KFD modules

`furyctl` can be used as a package manager for the KFD.
It providers a simple way to download all the desired modules of the KFD by reading a single `Furyfile`.

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

Each module is composed by a set of packages. In the previous `Furyfile`, we downloaded every module's packages. You can cherry-pick single packages using the `module/package` syntax.

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

You can find out what packages are inside each module by referring to each module documentation.

### 2. Download the modules

Run `furyctl vendor` (within the same directory where your `Furyfile` is located) to download the modules.

`furyctl` will download all the packages in a `vendor/` directory.

> 💡 **TIP**
>
> Use the `-H` flag in the `furyctl vendor` command to download using HTPP(S) instead of the default SSH. This is useful if you are in an environment that restricts the SSH traffic.

## Self-Provisioning

The self-provisioning feature is available via two commands:

- `furyctl bootstrap`: creates the required networking infrastructure
- `furyctl cluster`: creates a Fury cluster.

Both commands provide the following subcommands:

- `furyctl {bootstrap,cluster} template --provisioner {provisioner_name}`: Creates a `yml` configuration file with some default options making easy replacing these with the right values.
- `furyctl {bootstrap,cluster} init`: Initializes the project that deploys the infrastructure.
- `furyctl {bootstrap,cluster} apply`: Actually creates or updates the infrastructure.
- `furyctl {bootstrap,cluster} destroy`: Destroys the infrastructure.

All these three subcommands accept the following options:

```bash
-c, --config string:   Configuration file path
-t, --token string:    GitHub token to access enterprise repositories. Contact sales@sighup.io
-w, --workdir string:  Working directory with all project files
```

`apply` subcommand also implements the following option:

```bash
--dry-run: Dry run execution
```

### Anatomy of the configuration file

The self-provisioning feature uses a different configuration file than the `Furyfile.yml`.
While the `Furyfile.yml` file is used by the package-manager features, the self-provision features use a separated `cluster.yml` file:

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

### Workflow to deploy a cluster from zero

The following workflow describes a complete setup of a cluster from scratch.
The bootstrap command will create the underlay requirements to deploy a Kubernetes cluster. Most of these
components are network-related stuff.

Once the bootstrap process is up to date, the cluster command can be triggered using outputs from the
`bootstrap apply` command.

```bash
+--------------------------+   +--------------------------+   +--------------------------+   +--------------------------+
| furyctl bootstrap init   +-->+ furyctl bootstrap apply  +-->+ furyctl cluster init     +-->+ furyctl cluster apply   |
+--------------------------+   +--------------------------+   +--------------------------+   +--------------------------+
```

### Workflow to deploy a cluster from an already existing infrastructure

The following workflow describes a setup of a cluster using an already existing underlay infrastructure.

```bash
+--------------------------+   +--------------------------+
+ furyctl cluster init     +-->+ furyctl cluster apply    |
+--------------------------+   +--------------------------+
```

### Provisioners

To deploy all the components, `furyctl` introduces a new concept: `provisioners`.
These provisioners are terraform projects integrated with the `furyctl` binary. They can be open (like
the cluster EKS provisioner) or enterprise only (like the vSphere cluster, contact sales@sighup.io)

To use an **enterprise** provisioner, you need to specify a token in the
`furyctl {bootstrap,cluster} {init,apply,destroy} --token YOUR_TOKEN` commands.

> You can use an environment variable to avoid passing the token via console: `FURYCTL_TOKEN`.

Contact [sales@sighup.io](mailto:sales@sighup.io) to get more details about this feature.

#### Bootstrap

The current list of available `bootstrap` provisioners are:

- `aws`: It creates a VPC with all the requirements to deploy a Kubernetes Cluster. It also includes
a VPN instance easily manageable by using `furyagent`.
- `gcp`: It creates a Network with all the requirements to deploy a Kubernetes Cluster. It also
includes a VPN instance easily manageable by using `furyagent`.

#### Clusters

The current list of available `cluster` provisioners are:

- `eks`: It creates an EKS cluster on an already existing VPC. It uses the already existing
[fury-eks-installer](https://github.com/sighupio/fury-eks-installer) terraform code.
- `gke`: It creates an GKE cluster on an already existing Network. It uses the already existing
[fury-gke-installer](https://github.com/sighupio/fury-gke-installer) terraform code.
- `vsphere` **(enterprise)**: It creates a Kubernetes cluster on an already existing vSphere cluster.

#### Additional details

If you want to understand how to integrate more provisioners, read the [`CONTRIBUTING.md`](CONTRIBUTING.md) file.

To better understand how to use this self-provisioning feature take a look at the official Fury [documentaton site](https://kubernetesfury.com).

## License

Furyctl is an open-source software and it's released under the following [LICENSE](LICENSE)
