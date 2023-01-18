<!-- markdownlint-disable MD033 -->
<h1 align="center">
  <img src="docs/assets/furyctl-logo.svg" width="200px" alt="furyctl logo" />

<p>The multi-purpose command line tool<br/>for the Kubernetes Fury Distribution</p>

[![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg)](https://ci.sighup.io/sighupio/furyctl)
![Release](https://img.shields.io/github/v/release/sighupio/furyctl?label=Furyctl)
![Slack](https://img.shields.io/badge/slack-@kubernetes/fury-yellow.svg?logo=slack)
![License](https://img.shields.io/github/license/sighupio/furyctl)
[![Go Report Card](https://goreportcard.com/badge/github.com/sighupio/furyctl)](https://goreportcard.com/report/github.com/sighupio/furyctl)

</h1>
<!-- markdownlint-eable MD033 -->

<!-- <KFD-DOCS> -->

Furyctl is a command line interface tool to:

- create and manage Fury clusters on AWS
- download and manage the Kubernetes Fury Distribution (KFD) modules

<br/>

![Furyctl usage](docs/assets/furyctl.gif)

> 💡 Learn more about the Kubernetes Fury Distribution in the [official site](https://kubernetesfury.com).

## Installation

### Installation from binaries

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

To download the latest release, run:

```bash
wget -q "https://github.com/sighupio/furyctl/releases/download/v0.10.0/furyctl-$(uname -s)-amd64" -O /tmp/furyctl
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
INFO[0000] Furyctl version 0.10.0
```

> 💡 **TIP**
>
> Enable autocompletion for `furyctl` CLI on your shell (currently autocompletion is provided for `bash`, `zsh`, `fish`).
> To see the instruction on how to enable it, run `furyctl completion -h`

## Usage

See the available commands with `furyctl --help`:

```bash
furyctl --help

The multi-purpose command line tool for the Kubernetes Fury Distribution.

Furyctl is a simple CLI tool to:

- download and manage the Kubernetes Fury Distribution (KFD) modules
- create and manage Kubernetes Fury clusters

Usage:
  furyctl [command]

Available Commands:
  completion  Generate completion script
  create      Create a cluster or a config file
  delete      Delete a cluster
  download    Download all dependencies from the Kubernetes Fury Distribution specified in the config file
  dump        Dump templates and other useful fury objects
  help        Help about any command
  validate    Validate the config file and the dependencies relative to the Kubernetes Fury Distribution specified in it
  version     Print the version number of furyctl

Flags:
  -D, --debug               Enables furyctl debug output
  -d, --disable-analytics   Disable analytics
  -h, --help                help for furyctl
  -l, --log string          Path to the log file or stdout to log to standard output (default: ~/.furyctl/furyctl.log)
  -T, --no-tty              Disable TTY
  -w, --workdir string      Switch to a different working directory before executing the given subcommand.

Use "furyctl [command] --help" for more information about a command.
```

## Download and manage KFD modules

`furyctl` can be used as a package manager for the KFD.
It provides a simple way to download all the desired modules of the KFD by reading a single `furyctl.yaml`.

The process requires the following steps:

1. Generate a `furyctl.yaml` by running `furyctl create config` specifying the desired Kubernetes Fury Distribution version
   with the flag `--version`.
2. Run `furyctl download dependencies` to download all the dependencies including the modules of the KFD.

### 1. Customize the `furyctl.yaml`

A `furyctl.yaml` is a YAML formatted file that contains all the information needed to create a Kubernetes Fury cluster.

Modules are located in the `distribution` section of the `furyctl.yaml` file and can be configured to better fit your needs.

### 2. Download the modules

Run `furyctl download dependencies` (within the same directory where your `furyctl.yaml` is located) to download the modules and all the dependencies
needed to create a Kubernetes Fury cluster.

> 🔥 **Advanced User**
>
> Using the command `furyctl dump template` with the flag `-w` pointing to the local location of the repository `fury-distribution`,
> will run the template engine on the modules and generate the final manifests that will be applied to the cluster.

## Cluster creation

The Cluster creation feature is available via the command `furyctl create cluster`.

The subcommand accept the following options:

```bash
-b, --bin-path string      Path to the bin folder where all dependencies are installed
-c, --config string        Path to the furyctl.yaml file (default "furyctl.yaml")
--distro-location string   Location where to download schemas, defaults and the distribution manifest. It can either be a local path(eg: /path/to/fury/distribution) or a remote URL(eg: git::git@github.com:sighupio/fury-distribution?ref=BRANCH_NAME). Any format supported by hashicorp/go-getter can be used.
--dry-run                  Allows to inspect what resources will be created before applying them
-h, --help                 help for cluster
--kubeconfig string        Path to the kubeconfig file, mandatory if you want to run the distribution phase and the KUBECONFIG environment variable is not set
-p, --phase string         Limit the execution to a specific phase. options are: infrastructure, kubernetes, distribution
--skip-deps-download       Skip downloading the distribution modules, installers and binaries
--skip-deps-validation     Skip validating dependencies
--skip-phase string        Avoid executing a unwanted phase. options are: infrastructure, kubernetes, distribution. More specifically:
                           - skipping infrastructure will execute kubernetes and distribution
                           - skipping kubernetes will only execute distribution
                           - skipping distribution will execute infrastructure and kubernetes
--vpn-auto-connect         When set will automatically connect to the created VPN in the infrastructure phase

Global Flags:
-D, --debug               Enables furyctl debug output
-d, --disable-analytics   Disable analytics
-l, --log string          Path to the log file or stdout to log to standard output (default: ~/.furyctl/furyctl.log)
-T, --no-tty              Disable TTY
-w, --workdir string      Switch to a different working directory before executing the given subcommand.
```

### Deploy a cluster from zero

The following steps will guide you through the process of creating a Kubernetes Fury cluster from zero.

1. Follow the previous steps to generate a `furyctl.yaml` and download the modules.
2. Edit the `furyctl.yaml` to customize the cluster configuration by filling the sections `infrastructure`, `kubernetes` and `distribution`.
3. Check that the configuration file is valid by running `furyctl validate config`.
4. Run `furyctl create cluster` to create the cluster.
5. (Optional) Watch the logs of the cluster creation process with `tail -f ~/.furyctl/furyctl.log`.

> 💡 **Alpha ONLY**
>
> You may need to use the flag `--distro-location git::git@github.com:sighupio/fury-distribution.git?ref=feature/furyctl-next` until the next release of the KFD.

### Deploy a cluster from an already existing infrastructure

Same as the previous section, but you can skip the infrastructure creation phase
by not filling the section `infrastructure` in the `furyctl.yaml` file and
running `furyctl create cluster --skip-phase infrastructure`.

### Deploy a cluster step by step

The cluster creation process can be split into three phases:

- Infrastructure
- Kubernetes
- Distribution

The `furyctl create cluster` command will execute all the phases by default,
but you can limit the execution to a specific phase by using the flag `--phase`.

So in order to create a cluster step by step, you can run the following commands:

```bash
furyctl create cluster --phase infrastructure
```

If you choose to create a VPN in the infrastructure phase, you can automatically connect to it by using the flag `--vpn-auto-connect`.

```bash
furyctl create cluster --phase kubernetes
```

After running the command, remember to export the `KUBECONFIG` environment variable to point to the generated kubeconfig file or
to use the flag `--kubeconfig` in the following command.

```bash
furyctl create cluster --phase distribution
```

#### Infrastructure

The available `infrastructure` provisioners are:

| Provisioner | Description                                                                                                                                        |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `aws`       | It creates a VPC with all the requirements to deploy a Kubernetes Cluster. It also includes a VPN instance easily manageable by using `furyagent`. |

#### Kubernetes

The available `kubernetes` provisioners are:

| Provisioner | Description                                                                                                                         |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `eks`       | Creates an EKS cluster on an already existing VPC. It uses the [fury-eks-installer](https://github.com/sighupio/fury-eks-installer) |

<!-- </KFD-DOCS> -->
<!-- <FOOTER> -->

## Contributing

Before contributing, please read first the [Contributing Guidelines](docs/CONTRIBUTING.md).

## Test classes

There are four kind of tests: unit, integration, e2e, and expensive.

Each of them covers specific use cases depending on the speed, cost, and dependencies at play in a given scenario.
Anything that uses i/o should be marked as integration, with the only expection of local files and folders: any test
that uses the local filesystem and nothing more can be marked as 'unit'. This is made for convenience and it's open to
change in the future should we decide to refactor the code to better isolate that kind of i/o from the logic of the tool.

That said, here's a little summary of the used tags:

- unit: tests that exercise a single component or function in isolation. Tests using local files and dirs are permitted here.
- integration: tests that require external services, such as github. Test using only local files and dirs should not be marked as integration.
- e2e: tests that exercise furyctl binary, invoking it as a cli tool and checking its output
- expensive: e2e tests that incur in some monetary cost, like running an EKS instance on AWS

### Reporting Issues

In case you experience any problems, please [open a new issue](https://github.com/sighupio/furyctl/issues/new/choose).

## License

This module is open-source and it's released under the following [LICENSE](LICENSE)

<!-- </FOOTER> -->
