# Furyctl

Furyctl is the package manager for Fury distribution. It’s simple to use and reads a single Furyfile to download
packages you need. Fury distribution offers three types of packages:

- **Bases** : Sets of Kustomize bases to deploy necessary components in Kubernetes
- **Modules**: Terraform modules to deploy Kubernetes infrastructure, and it’s dependencies
- **Roles**: Ansible roles for deploying, configuring, and managing a Kubernetes infrastructure

In addition to the package manager feature, it enables you to self-provision Fury Clusters.
Read more about this feature on its documentation site.

## Furyfile

Furyfile is a simple YAML formatted file where you list which packages(and versions) you want to have.
You can omit a type if you don't need any of its packages. An example Furyfile with packages listed
would be like the following:

```yaml
# all sections are optional

# map of prefixes and versions used to force a specific version for all the matching roles/modules/bases
versions:
  # e.g. will force version v1.15.4 if the name matches "aws*"
  aws: v1.15.4
  monitoring: master

roles:
  - name: aws/etcd
  - name: aws/kube-control-plane

modules:
  - name: aws/aws-vpc
  - name: aws/aws-kubernetes

bases:
  - name: monitoring
  - name: logging
  # versions can be overridden if needed by specifying them for each package
    version: master
```

You can get all packages in a group by using a group name *(like `logging`)* or single packages under a group
(like `monitoring/prometheus-operator`).

## Install

### Github Releases

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

Supported architectures are *(64 bit)*:
- `linux`
- `darwin`

Download the right binary for your architecture, then add it to your `PATH`. Assuming it's downloaded in your
`~/Downloads` folder, you can run following commands (replacing `{arch}` with your architecture):

```bash
chmod +x ~/Downloads/furyctl-{arch}-amd64 && mv ~/Downloads/furyctl-{arch}-amd64 /usr/local/bin/furyctl
```

### Homebrew

If you are a macOS user:

```bash
brew tap sighupio/furyctl
brew install furyctl
```

## Usage

### Package Manager

- Once you installed furyctl binary you can see available commands with `furyctl --help`:

```bash
$ furyctl --help

A command line tool to manage cluster deployment with Kubernetes

Usage:
  furyctl [command]

Available Commands:
  bootstrap   Creates the required infrastructure to deploy a battle-tested Kubernetes cluster, mostly network components
  cluster     Creates a battle-tested Kubernetes cluster
  help        Help about any command
  init        Initialize the minimum distribution configuration
  vendor      Download dependencies specified in Furyfile.yml
  version     Prints the client version information

Flags:
      --debug   Enables furyctl debug output
  -h, --help    help for furyctl

Use "furyctl [command] --help" for more information about a command.
```

- To download the minimal Kubernetes Fury Distribution files (within the same directory) you can run `furyctl init` command:
```bash
$ furyctl init --version v1.0.0
2020/02/05 09:48:05 downloading: http::https://github.com/sighupio/poc-fury-distribution/releases/download/1.0.0/Furyfile.yml -> Furyfile.yml
2020/02/05 09:49:05 downloading: http::https://github.com/sighupio/poc-fury-distribution/releases/download/1.0.0/kustomization.yaml -> kustomization.yaml
```

- To download packages, you can run `furyctl vendor` (within the same directory where your Furyfile is located):

```bash
$ furyctl vendor
2020/02/05 10:49:47 using v1.15.4 for package aws/etcd
2020/02/05 10:49:47 using v1.15.4 for package aws/kube-control-plane
2020/02/05 10:49:47 using v1.15.4 for package aws/aws-vpc
2020/02/05 10:49:47 using v1.15.4 for package aws/aws-kubernetes
2020/02/05 10:49:47 using master for package monitoring
2020/02/05 10:49:47 downloading: git@github.com:sighupio/fury-kubernetes-aws//roles/kube-control-plane?ref=v1.15.4 -> vendor/roles/aws/kube-control-plane
2020/02/05 10:49:47 downloading: git@github.com:sighupio/fury-kubernetes-aws//modules/aws-kubernetes?ref=v1.15.4 -> vendor/modules/aws/aws-kubernetes
2020/02/05 10:49:47 downloading: git@github.com:sighupio/fury-kubernetes-monitoring//katalog?ref=master -> vendor/katalog/monitoring
2020/02/05 10:49:47 downloading: git@github.com:sighupio/fury-kubernetes-aws//modules/aws-vpc?ref=v1.15.4 -> vendor/modules/aws/aws-vpc
2020/02/05 10:49:47 downloading: git@github.com:sighupio/fury-kubernetes-aws//roles/etcd?ref=v1.15.4 -> vendor/roles/aws/etcd
2020/02/05 10:49:49 downloading: git@github.com:sighupio/fury-kubernetes-logging//katalog?ref=master -> vendor/katalog/logging
```
You will find your packages under `vendor/{roles,modules,katalog}` directories created where you executed `furyctl`.

- You can get furyctl version with `furyctl version`:

```bash
$ furyctl version
INFO[0000] Furyctl version 0.2.3
```

### Self-Provisioning

The self-provisioning feature is available with two commands:

- `furyctl bootstrap`: Use it to create the required infrastructure to place the cluster. Skip it if you
already managed to have passed all the cluster requirements.
- `furyctl cluster`: Deploys a Fury cluster.

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

#### Anatomy of the configuration file

The self-provisioning feature uses a different configuration file than the `Furyfile.yml`.
Use the `Furyfile.yml` file while using package-manager features.

```yaml
kind: # Cluster or Bootstrap
metadata:
  name: # Name of the deployment. It can be used by the provisioners as a unique identifier.
executor: # This is an optional attribute. It defines the terraform executor to use along with the backend configuration
  version: # Optional attribute. Terraform version to use. Default is latest
  state: # Optional attribute. It configures the backend configuration file.
    backend: # Optional attribute. It configures the backend to use. Default to local
    config: # Optional attribute. It configures the configuration of the selected backend configuration. It accepts multiple key values.
      # bucket: "my-bucket" # Example
      # key: "terraform.tfvars"
      # region: "eu-home-1" # Example
provisioner: # Defines what provisioner to use.
spec: {} # Input variables of the provisioner. Read each provisioner definition to understand what are the valid values.
```

#### Workflow to deploy a cluster from zero

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

#### Workflow to deploy a cluster from an already existing infrastructure

The following workflow describes a setup of a cluster using an already existing underlay infrastructure.

```bash
+--------------------------+   +--------------------------+
+ furyctl cluster init     +-->+ furyctl cluster apply    |
+--------------------------+   +--------------------------+
```

#### Provisioners

To deploy all the components, `furyctl` introduces a new concept: `provisioners`.
These provisioners are terraform projects integrated with the `furyctl` binary. They can be open (like
the cluster EKS provisioner) or enterprise only (like the bootstrap AWS, contact sales@sighup.io)

To use an **enterprise** provisioner, you need to specify a token in the
`furyctl {bootstrap,cluster} {init,apply,destroy} --token YOUR_TOKEN` commands.

> You can use an environment variable to avoid passing the token via console: `FURYCTL_TOKEN`.

Contact [sales@sighup.io](mailto:sales@sighup.io) to get more details about this feature.

##### Bootstrap

The current list of available `bootstrap` provisioners are:

- `aws` **(enterprise)**: It creates a VPC with all the requirements to deploy a Kubernetes Cluster. It also includes
a VPN instance easily manageable by using `furyagent`.
- `gcp` **(enterprise)**: It creates a Network with all the requirements to deploy a Kubernetes Cluster. It also
includes a VPN instance easily manageable by using `furyagent`.

##### Clusters

The current list of available `cluster` provisioners are:

- `eks`: It creates an EKS cluster on an already existing VPC. It uses the already existing
[fury-eks-installer](https://github.com/sighupio/fury-eks-installer) terraform code.
- `gke`: It creates an GKE cluster on an already existing Network. It uses the already existing
[fury-gke-installer](https://github.com/sighupio/fury-gke-installer) terraform code.

#### Additional details

If you want to understand how to integrate more provisioners, read the [`CONTRIBUTING.md`](CONTRIBUTING.md) file.
On the other side, to better understand how to use this self-provisioning feature take a look at the official Fury
[documentaton site](https://kubernetesfury.com).
