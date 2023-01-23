<!-- markdownlint-disable MD033 -->
<h1 align="center">
  <img src="docs/assets/furyctl-logo.svg" width="200px" alt="furyctl logo" />

<p>The Swiss Army Knife<br/>for the Kubernetes Fury Distribution</p>

<!-- FIXME: UPDATE THE BUILD BADGE WITH THE RIGHT BRANCH -->
[![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg?ref=refs/heads/furyctl-ng-alpha1)](https://ci.sighup.io/sighupio/furyctl)
![Release](https://img.shields.io/badge/furyctl%20Next%20Generation-alpha1-blue)
![Slack](https://img.shields.io/badge/slack-@kubernetes/fury-yellow.svg?logo=slack)
![License](https://img.shields.io/github/license/sighupio/furyctl)

</h1>
<!-- markdownlint-eable MD033 -->

<!-- <KFD-DOCS> -->

`furyctl` is the command line companion for the Kubernetes Fury Distribution to manage the **full lifecycle** of your Kubernetes Fury clusters.
<br/>
<!-- line left blank -->
> 💡 Learn more about the Kubernetes Fury Distribution in the [official site](https://kubernetesfury.com).
<!-- line left blank -->
> **Warning** you are viewing the readme for furyctl next generation (`furyctl-ng` for short). This version is in `alpha` status.
>
> `furyctl-ng` currently has support for EKS-based clusters only.

## Installation

### Installation from source

Prerequisites:

- `make`
- `go == v1.19`
- `goreleaser == v1.11.4`

To build `furyctl` from source, follow the next steps:

1. clone the repository:

<!-- FIXME: remove the branch swithing in the future -->
```console
git clone git@github.com:sighupio/furyctl.git
# cd into the cloned repository
cd furyctl
# Switch to the branch for the `furyctl-ng-alpha1` version
git switch furyctl-ng-alpha1
```

2. build the binaries by running the following command:

```console
make build
```

3. you will find the binaries for Linux, Darwin (macOS) and Windows for your current architecture in the `dist` folder:

```console
$ tree dist/furyctl_*/
dist/furyctl_darwin_amd64_v1
└── furyctl
dist/furyctl_linux_amd64_v1
└── furyctl
dist/furyctl_windows_amd64_v1
└── furyctl.exe
```

4. check that the binary is working as expected:

```console
# replace darwin with your OS and amd64 with your architecture
./dist/furyctl_darwin_amd64_v1/furyctl version
```

5. (optional) copy the binary to your `bin` folder:

```console
# replace darwin with your OS and amd64 with your architecture
sudo mv ./dist/furyctl_darwin_amd64_v1/furyctl /usr/local/bin/furyctl
```

### Installation from binaries (not available yet for `furyctl-ng`)

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

To download the latest release, run:

```console
wget -q "https://github.com/sighupio/furyctl/releases/download/v0.9.0/furyctl-$(uname -s)-amd64" -O /tmp/furyctl
chmod +x /tmp/furyctl
sudo mv /tmp/furyctl /usr/local/bin/furyctl
```

Alternatively, you can install `furyctl` using a brew tap or via an asdf plugin.

> ⚠️ M1 users: please download `darwin/amd64` binaries instead of using homebrew or asdf. Even though furyctl can be build for `arm64`, some of its dependendecies are not available yet for this architecture.

### Installation with [Homebrew](https://brew.sh/) (not available yet for `furyctl-ng`)

```console
brew tap sighupio/furyctl
brew install furyctl
```

### Installation with [asdf](https://github.com/asdf-vm/asdf) (not available yet for `furyctl-ng`)

Add furyctl asdf plugin:

```console
asdf plugin add furyctl
```

Check that everything is working correctly with `furyctl version`:

```console
$ furyctl version
buildTime: 2023-01-13T09:50:15Z
gitCommit: 349c14a06dd6163b308e4e8baa47ec9cc59712e1
goVersion: go1.19
osArch: amd64
version: 0.9.0
```

## Usage

See all the available commands and their usage by running `furyctl help`.

> 💡 **TIP**
>
> Enable command tab autocompletion for `furyctl` on your shell (`bash`, `zsh`, `fish` are supported).
> See the instruction on how to enable it with `furyctl completion --help`

<!-- line left blank as spacer -->

> **Warning**
> furyctl-ng alpha only
>
> `furyctl-ng` is compatible with KFD versions 1.22.1, 1.23.3 and 1.24.0, but you will need to use the flag `--distro-location 'git::git@github.com:sighupio/fury-distribution.git?depth=1&ref=feature/furyctl-next'`
> in *every command* until the next release of the KFD.

### Basic Usage

Basic usage of `fuyrctl` for a new project consists on the following steps:

1. Creating a configuration file defining the prequired infrastructure, Kubernetes cluster details, and KFD modules configuration.
2. Creating a cluster as defined in the configuration file.
3. Destroying the cluster and its related resources.

#### 1. Create a configuration file

`furyctl` provides a command that outputs a sample configuration file (by default called `furyctl.yaml`) with all the possible fields commented.
To create a sample configuration file as a starting point use the following command:

```console
furyctl create config --version <KFD version> 
```

> 💡 **TIP**
>
> You can pass some additional flags, like the kind of cluster or the configuration file name.
>
> See `furyctl create config --help` for more details.

Open the generated configuration file and edit it according to your needs. You can follow the instructions included as comments in the file.

furyctl's configuration files have a kind, that specifies what type of cluster will be created, for example the `EKSCluster` kind has all the parameters needed to create a KFD cluster using the EKS managed clusters from AWS.

Additionaly, the schema of the file is versioned with the `apiVersion` field, so when new features are introduced you can switch to a newer version of the configuration file structure.

Once you have filled your configuration file, you can check that it's content is valid by running the following comand:

```console
furyctl validate config --config <path to your config file>
```

> **Note** the `--config` flag is optional, set it if your configuration file is not named `furyctl.yaml`

#### 2. Create a cluster

In the previous step, you have created and validated a configuration file that defines the cluster and its sorroundings, you can now proceed to actually creating the resources.

furcytl has divided the cluster creation in three phases: `infrastructure`, `kubernetes` and `distribution`.

- The first phase, `infrastructure`, creates all the prerequisites needed to be able to create a cluster. For example, the VPC and its networks.
- The second phase, `kubernetes`, creates the actual Kubernetes clusters. For example, the EKS cluster with its node pools.
- The third phase, `distribution`, deploys KFD modules to the Kubernetes cluster.

> 💡 You may find these phases familiar from editing the configuration file.

Just like you can validate that your configuration file is well formed. `furyctl` let's you check that you have all the needed dependencies (environment variables, binaries, etc.) before starting a cluster creation process.

To validate that you have all the dependencies needed to create the cluster defined in your configuration file, run the following command:

```console
furyctl validate dependencies
```

Finally, to launch the creation of the resources defined in the configuration file, run the following command:

> **Warning** you are about to create cloud resources that could have billing impact.

```console
furyctl create cluster
```

> **Note** the creation process can take a while.

🎉 Congratulations! You have created your production-grade Kubernetes Fury Cluster from scratch and it's ready to go into battle.

#### 3. Destroy a cluster

Destroying a cluster can be thought as running the creation phases in reverse order. `furyctl` automates this operation for you.
To destroy a cluster created using `furyctl` and all its related resources, run the following command:

> **Warning** you are about to run a destructive operation.

```console
furyctl delete cluster --dry-run
```

check that the dry-run output is what you expected and then run the command again without the `--dry-run` flag to actually delete all the resources.

> 💡 **TIP**
>
> Notice the `--dry-run` flag, used to first check what the command would do. This flag is available for other commands too.

### Advanced Usage

#### Download and manage KFD modules

`furyctl` can be used as a package manager for KFD.

It provides a simple way to download all the desired modules of the KFD by reading a single `furyctl.yaml`.

The process requires the following steps:

1. Generate a `furyctl.yaml` by running `furyctl create config` specifying the desired Kubernetes Fury Distribution version
   with the flag `--version`.
2. Run `furyctl download dependencies` to download all the dependencies including the modules of the KFD.

##### 1. Customize the `furyctl.yaml`

A `furyctl.yaml` is a YAML formatted file that contains all the information needed to create a Kubernetes Fury cluster.

Modules are located in the `distribution` section of the `furyctl.yaml` file and can be configured to better fit your needs.

##### 2. Download the modules

Run `furyctl download dependencies` (within the same directory where your `furyctl.yaml` is located) to download the modules and all the dependencies
needed to create a Kubernetes Fury cluster.

> 🔥 **Advanced User**
>
> Using the command `furyctl dump template` with the flag `-w` pointing to the local location of the repository `fury-distribution`,
> will run the template engine on the modules and generate the final manifests that will be applied to the cluster.

#### Cluster creation

The following steps will guide you through the process of creating a Kubernetes Fury cluster from zero.

1. Follow the previous steps to generate a `furyctl.yaml` and download the modules.
2. Edit the `furyctl.yaml` to customize the cluster configuration by filling the sections `infrastructure`, `kubernetes` and `distribution`.
3. Check that the configuration file is valid by running `furyctl validate config`.
4. Run `furyctl create cluster` to create the cluster.
5. (Optional) Watch the logs of the cluster creation process with `tail -f ~/.furyctl/furyctl.log`.

#### Create a cluster in an already existing infrastructure

Same as the previous section, but you can skip the infrastructure creation phase
by not filling the section `infrastructure` in the `furyctl.yaml` file and
running `furyctl create cluster --skip-phase infrastructure`.

#### Deploy a cluster step by step

The cluster creation process can be split into three phases:

- Infrastructure
- Kubernetes
- Distribution

The `furyctl create cluster` command will execute all the phases by default,
but you can limit the execution to a specific phase by using the `--phase` flag.

To create a cluster step by step, you can run the following command:

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

##### Infrastructure

The available `infrastructure` provisioners are:

| Provisioner | Description                                                                                                                                        |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `aws`       | It creates a VPC with all the requirements to deploy a Kubernetes Cluster. It also includes a VPN instance easily manageable by using `furyagent`. |

##### Kubernetes

The available `kubernetes` provisioners are:

| Provisioner | Description                                                                                                                         |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `eks`       | Creates an EKS cluster on an already existing VPC. It uses the [fury-eks-installer](https://github.com/sighupio/fury-eks-installer) |

<!-- </KFD-DOCS> -->
<!-- <FOOTER> -->

## Contributing

Before contributing, please read first the [Contributing Guidelines](docs/CONTRIBUTING.md).

### Test classes

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

In case you experience any problems with `furyctl`, please [open a new issue](https://github.com/sighupio/furyctl/issues/new/choose) in GitHub.

## License

This software is open-source and it's released under the following [LICENSE](LICENSE).

<!-- </FOOTER> -->
