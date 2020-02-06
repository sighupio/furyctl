## Furyctl

Furyctl is package manager for Fury distribution. It’s simple to use and reads a single Furyfile to download
packages you need. Fury distribution offers three types of packages:

- **Bases** : Sets of Kustomize bases to deploy basic components in Kubernetes
- **Modules**: Terraform modules to deploy kubernetes infrastructure and it’s dependencies
- **Roles**: Ansible roles for deploying, configuring and managing a Kubernetes infrastructure

### Furyfile

Furyfile is a simple YAML formatted file where you list which packages(and versions) you want to have.
You can omit a type if you don't need any of its packages. An example Furyfile with packages listed
would be like following:

```
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

You can get all packages in a group by using group name (like `logging`) or single packages under a group
(like `monitoring/prometheus-operator`).

### Install

#### Github Releases

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

Supported architectures are (64 bit):
- `linux`
- `darwin`

Download right binary for your architecture and add it to your PATH. Assuming it's downloaded in your
`~/Downloads` folder, you can run following commands (replacing `{arch}` with your architecture):

```
chmod +x  ~/Downloads/furyctl-{arch}-amd64 && mv ~/Downloads/furyctl-{arch}-amd64 /usr/local/bin/furyctl
```

#### Homebrew

If you are a macOS user:

```bash
$ brew tap sighupio/furyctl
$ brew install furyctl
```

### Usage

- Once you installed furyctl binary you can see available commands with `furyctl --help`:

```bash
$ furyctl --help

A command line tool to manage cluster deployment with kubernetes

Usage:
  furyctl [command]

Available Commands:
  help         Help about any command
  init         Initialize the minimum distribution configuration
  vendor       Download dependencies specified in Furyfile.yml
  version      Prints the client version information

Flags:
  -h, --help     help for furyctl
  -t, --toggle   Help message for toggle

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
You will find your packages under `vendor/{roles,modules,katalog}` directories created where you called `furyctl`.

- You can get furyctl version with `furyctl version`:

```bash
$ furyctl version
2020/02/06 13:44:44 Furyctl version  0.1.7
```
