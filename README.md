<!-- markdownlint-disable MD033 -->
<h1 align="center">
  <!-- Using a temporary PNG until we get the updated SVG -->
  <!-- <img src="docs/assets/furyctl-logo.svg" width="200px" alt="furyctl logo" /> -->
  <img src="docs/assets/furyctl-temporary.png" width="200px" alt="furyctl logo" />

<p>The Swiss Army Knife<br/>for the Kubernetes Fury Distribution</p>

<!-- FIXME: UPDATE THE BUILD BADGE WITH THE RIGHT BRANCH -->

[![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg)](https://ci.sighup.io/sighupio/furyctl)
![Release](https://img.shields.io/badge/furyctl-v0.27.3-blue)
![Slack](https://img.shields.io/badge/slack-@kubernetes/fury-yellow.svg?logo=slack)
![License](https://img.shields.io/github/license/sighupio/furyctl)
[![Go Report Card](https://goreportcard.com/badge/github.com/sighupio/furyctl)](https://goreportcard.com/report/github.com/sighupio/furyctl)

</h1>
<!-- markdownlint-eable MD033 -->

<!-- <KFD-DOCS> -->

> The next generation of `furyctl`, called "furyctl next", has been officially released. It is now in a stable state and available starting from version v0.25.0. The previous version, furyctl 0.11, is considered legacy and will only receive bug fixes. It will be maintained under the v0.11 branch.

## What is Furyctl?

`furyctl` is the command line companion for the Kubernetes Fury Distribution to manage the **full lifecycle** of your Kubernetes Fury clusters.
<br/>

> 💡 Learn more about the Kubernetes Fury Distribution in the [official site](https://kubernetesfury.com).

If you're looking for the old documentation, you can find it [here](https://github.com/sighupio/furyctl/blob/release-v0.11/README.md).

### Available providers

- `EKSCluster`: Provides comprehensive lifecycle management for an EKS cluster on AWS. It handles the installation of the VPC, VPN, EKS using the installers, and deploys the Distribution onto the EKS cluster.
- `KFDDistribution`: Dedicated provider for the distribution, which installs the Distribution (modules only) on an existing Kubernetes cluster.
- `OnPremises`: Provider to install a KFD Cluster on VMs.

## Support & Compatibility 🪢

Check the [compatibility matrix][compatibility-matrix] for additional information about `furyctl` and `KFD` compatibility.

## Installation

### Installing from binaries

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

To download the latest release, run:

```console
curl -L "https://github.com/sighupio/furyctl/releases/latest/download/furyctl-$(uname -s)-amd64.tar.gz" -o /tmp/furyctl.tar.gz && tar xfz /tmp/furyctl.tar.gz -C /tmp
chmod +x /tmp/furyctl
sudo mv /tmp/furyctl /usr/local/bin/furyctl
```

Alternatively, you can install `furyctl` using the asdf plugin.

<!--
### Installing with [Homebrew](https://brew.sh/)

```console
brew tap sighupio/furyctl
brew install furyctl
```
-->

### Installing with [asdf](https://github.com/asdf-vm/asdf)

Add furyctl asdf plugin:

```console
asdf plugin add furyctl
```

Check that everything is working correctly with `furyctl version`:

```console
$ furyctl version
...
goVersion: go1.22
osArch: amd64
version: 0.27.3
```

### Installing from source

Prerequisites:

- `make >= 4.1`
- `go >= 1.22`
- `goreleaser >= v1.24`

> You can install `goreleaser` with the following command once you have Go in your system:
>
> ```console
> go install github.com/goreleaser/goreleaser@v1.24.0
> ```

Once you've ensured the above dependencies are installed, you can proceed with the installation.

1. Clone the repository:

<!-- FIXME: remove the branch switching in the future -->

```console
git clone git@github.com:sighupio/furyctl.git
# cd into the cloned repository
cd furyctl
```

2. Build the binaries by running the following command:

```console
make build
```

3. You will find the binaries for Linux and Darwin (macOS) for your current architecture inside the `dist` folder:

```console
$ tree dist/furyctl_*/
dist/furyctl_darwin_amd64_v1
└── furyctl
dist/furyctl_linux_amd64_v1
└── furyctl
```

4. Check that the binary is working as expected:

> **Note** replace `darwin` with your OS and `amd64` with your architecture in the following commands.

```console
./dist/furyctl_darwin_amd64_v1/furyctl version
```

5. (optional) move the binary to your `bin` folder, in macOS:

```console
sudo mv ./dist/furyctl_darwin_amd64_v1/furyctl /usr/local/bin/furyctl
```

## Usage

See all the available commands and their usage by running `furyctl help`.

> 💡 **TIP**
>
> Enable command tab autocompletion for `furyctl` on your shell (`bash`, `zsh`, `fish` are supported).
> See the instruction on how to enable it with `furyctl completion --help`

<!-- line left blank as spacer -->

> Check [KFD Compatibility matrix](https://github.com/sighupio/fury-distribution/blob/main/docs/COMPATIBILITY_MATRIX.md) for the Furyctl / KFD versions to use.

### Basic Usage

Basic usage of `furyctl` for a new project consists on the following steps:

1. Creating a configuration file defining the prequired infrastructure, Kubernetes cluster details, and KFD modules configuration.
2. Creating a cluster as defined in the configuration file.
3. Destroying the cluster and its related resources.

#### 1. Create a configuration file

`furyctl` provides a command that outputs a sample configuration file (by default called `furyctl.yaml`) with all the possible fields explained in comments.

Furyctl configuration files have a kind that specifies what type of cluster will be created, for example the `EKSCluster` kind has all the parameters needed to create a KFD cluster using the EKS managed clusters from AWS.

You can also use the `KFDDistribution` kind to install the KFD distribution on top of an existing Kubernetes cluster or `OnPremises` kind to install a KFD cluster on VMs.

Additionaly, the schema of the file is versioned with the `apiVersion` field, so when new features are introduced you can switch to a newer version of the configuration file structure.

To scaffold a configuration file to use as a starter, you use the following command:

```console
furyctl create config --version v1.27.2 --kind "EKSCluster"
```

> 💡 **TIP**
>
> You can pass some additional flags, like the schema (API) version of the configuration file or a different configuration file name.
>
> See `furyctl create config --help` for more details.

Open the generated configuration file with your editor of choice and edit it according to your needs. You can follow the instructions included as comments in the file.

Once you have filled your configuration file, you can check that it's content is valid by running the following comand:

```console
furyctl validate config --config /path/to/your/furyctl.yaml
```

> 📖 **NOTE**
>
> The `--config` flag is optional, set it if your configuration file is not named `furyctl.yaml`

#### 2. Create a cluster

Requirements (EKSCluster):

- AWS CLI
- OpenVPN (when filling the `vpn` field in the configuration file)

In the previous step, you have created and validated a configuration file that defines the Kubernetes cluster and its sorroundings, you can now proceed to actually creating the resources.

Furyctl divides the cluster creation in four phases: `infrastructure`, `kubernetes`, `distribution` and `plugins`.

1. The first phase, `infrastructure`, creates all the prerequisites needed to be able to create a cluster. For example, the VPC and its networks.
2. The second phase, `kubernetes`, creates the actual Kubernetes clusters. For example, the EKS cluster and its node pools.
3. The third phase, `distribution`, deploys KFD modules to the Kubernetes cluster.
4. The fourth phase, `plugins`, installs Helm and Kustomize plugins into the cluster.

> 📖 **NOTE**
>
> You will find these four phases when editing the furyctl.yaml file.

Just like you can validate that your configuration file is well formed, `furyctl` let you check that you have all the needed dependencies (environment variables, binaries, etc.) before starting a cluster creation process.

To validate that your system has all the dependencies needed to create the cluster defined in your configuration file, run the following command:

```console
furyctl validate dependencies
```

Last but not least, you can launch the creation of the resources defined in the configuration file by running the following command:

> ❗️ **WARNING**
>
> You are about to create cloud resources that could have billing impact.

<!-- spacer -->

> 📖 **NOTE**
>
> The cluster creation process, by default, will create a VPN in the `infrastructure` phase and connect your machine to it automatically before proceeding to the `kubernetes` phase.

```console
furyctl create cluster --config /path/to/your/furyctl.yaml
```

> 💡 **TIP**
>
> You can use the alias `furyctl apply` instead of `furyctl create cluster`.

> 📖 **NOTE**
>
> The creation process will take a while.

🎉 Congratulations! You have created your production-grade Kubernetes Fury Cluster from scratch and it is now ready to go into battle.

#### 3. Upgrade a cluster

Upgrading a cluster is a process that can be divided into two steps: upgrading the fury version and running the migrations (if present).

The first step consist in bringing the cluster up to date with the latest version of the Kubernetes Fury Distribution. This is done by running the following command:

```console
furyctl apply --upgrade --config /path/to/your/furyctl.yaml
```

Once that is done, if you were also planning to move to a different provider (e.g.: `opensearch` to `loki`), you can run the following command to run the migrations:

```console
furyctl apply --config /path/to/your/furyctl.yaml
```

> ❗️ **WARNING**
>
> You must first upgrade the cluster using the old provider(e.g.: `opensearch`), update the configuration file to use the new provider(e.g.: `loki`) and then run the command above.

#### 3.1. Advanced upgrade options (OnPremises provider only)

You can also split nodes upgrade process into several steps, for example, you can upgrade the control plane nodes first:

```console
furyctl apply --upgrade --config /path/to/your/furyctl.yaml --skip-nodes-upgrade
```

And then upgrade the worker nodes, one by one:

```console
furyctl apply --upgrade --config /path/to/your/furyctl.yaml --upgrade-node workerNode1
```

At the end of the node upgrade process, a check is performed to ensure every pod is either `Running` or in a `Completed` state. You can specify a timeout for this check with the `--pod-running-check-timeout` flag or skip it with the `--force pods-running-check` flag.

#### 4. Destroy a cluster

Destroying a cluster will run through the four phases mentioned above, in reverse order, starting from `distribution`.

To destroy a cluster created using `furyctl` and all its related resources, run the following command:

> ❗️ **WARNING**
>
> You are about to run a destructive operation.

```console
furyctl delete cluster --dry-run
```

Check that the dry-run output is what you expect and then run the command again without the `--dry-run` flag to actually delete all the resources.

> 💡 **TIP**
>
> Notice the `--dry-run` flag, used to check what the command would do. This flag is available for other commands too.

### Advanced Usage

<!--

TODO This is not a viable way to manage dependencies without the possibility to change the --workdir instead of using ~/.furyctl

#### KFD modules management

`furyctl` can be used as a package manager for KFD.

It provides a simple way to download all the desired modules of the KFD by reading a single `furyctl.yaml`.

The process requires the following steps:

1. Generate a `furyctl.yaml` by running `furyctl create config` specifying the desired Kubernetes Fury Distribution version using the `--version` flag.
2. Run `furyctl download dependencies` to download all the dependencies including the modules of the KFD.

##### 1. Customizing the `furyctl.yaml`

A `furyctl.yaml` is a YAML-formatted file that contains all the information that is needed to create a Kubernetes Fury cluster.

Modules are located in the `distribution` section of the `furyctl.yaml` file and can be configured to better fit your needs.

##### 2. Downloading the modules

Run `furyctl download dependencies` (within the same directory where your `furyctl.yaml` is located) to download the modules and all the dependencies that are needed to create a Kubernetes Fury cluster.
-->

#### Cluster creation

The following steps will guide you through the process of creating a Kubernetes Fury cluster from zero.

1. Follow the previous steps to generate a `furyctl.yaml` and download the modules.
2. Edit the `furyctl.yaml` to customize the cluster configuration by filling the sections `infrastructure`, `kubernetes` and `distribution`.
3. Check that the configuration file is valid by running `furyctl validate config`.
4. Run `furyctl create cluster` to create the cluster.
5. (Optional) Watch the logs of the cluster creation process with `tail -f ~/.furyctl/furyctl.<timestamp>-<random>.log`.

#### Create a cluster in an already existing infrastructure

Same as the previous section, but you can skip the infrastructure creation phase
by not filling the section `infrastructure` in the `furyctl.yaml` file and
running `furyctl create cluster --start-from kubernetes`.

#### Deploy a cluster step by step

The cluster creation process can be split into three phases:

1. Infrastructure
2. Kubernetes
3. Distribution

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

#### Legacy download

The new furyctl still embed some of the legacy features, for example the command `furyctl legacy vendor` to download KFD dependencies from a deprecated `Furyfile.yml`.

This can be still used to manually manage all the components of the distribution.

> 💡 **TIP**
>
> you can also use `--furyfile` to point to a `Furyfile.yaml` in a different folder

#### Plugins

Furyctl supports Helm and Kustomize plugins.

##### Helm plugins

To install an Helm plugin (chart), first you have to add the repository to the `spec.plugins.helm.repositories` section of your `furyctl.yaml` file and then you can add the release to the `spec.plugins.helm.releases` section, specifying the chart name, the namespace, the chart version and the values to override. To override the values you can use the `spec.plugins.helm.releases[].set` or the `spec.plugins.helm.releases[].values` section.

For example to install the Prometheus Helm chart you have to add the following to your `furyctl.yaml`:

```yaml
...
spec:
  ...
  plugins:
    helm:
      repositories:
        - name: prometheus-community
          url: https://prometheus-community.github.io/helm-charts
      releases:
        - name: prometheus
          namespace: prometheus
          chart: prometheus-community/prometheus
          version: "24.3.0"
          set:
            - name: server.replicaCount
              value: 3
          values:
            - path/to/values.yaml
```

##### Kustomize plugins

To install a Kustomize plugin (project) you have to configure the `spec.plugins.kustomize` section of your `furyctl.yaml` file, specifying a name and the path to the folder.

For example:

```yaml
...
spec:
  ...
  plugins:
    kustomize:
      - name: kustomize-project
        folder: path/to/kustomize/project
```

### Advanced Tips

#### Using a custom distribution location

Furyctl comes with the flag `--distro-location`, allowing you to use a local copy of KFD instead of downloading it from the internet. This allows you to test changes to the KFD without having to push them to the repository, and might come in handy when you need to test new features or bugfixes.

#### Using a custom upgrade path location

On the same note, the tool comes with the `--upgrade-path-location` flag, too, allowing you to test changes to the upgrade path without having to push them to the repository, and to support cases that are not covered by the official release, such as upgrading from a beta or rc release to a stable one.

#### Restarting the cluster creation or update process from a specific (sub-)phase

If, for any reason, the cluster creation or update process fails, you can restart it from a specific (sub-)phase by using the `--start-from` flag. Starting from v0.27.0 we introduced the support for sub-phases, to give the operator more control over the process. The supported options are: `pre-infrastructure`, `infrastructure`, `post-infrastructure`, `pre-kubernetes`, `kubernetes`, `post-kubernetes`, `pre-distribution`, `distribution`, `post-distribution`, `plugins`.

<!-- </KFD-DOCS> -->
<!-- <FOOTER> -->

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

In case you experience any problems with `furyctl`, please [open a new issue](https://github.com/sighupio/furyctl/issues/new/choose) on GitHub.

## License

This software is open-source and it's released under the following [LICENSE](LICENSE).

<!-- </FOOTER> -->

[compatibility-matrix]: https://github.com/sighupio/furyctl/blob/main/docs/COMPATIBILITY_MATRIX.md
