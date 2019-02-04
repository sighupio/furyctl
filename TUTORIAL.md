## Furyctl Tutorial

Furyctl is package manager for Fury distribution. It’s simple to use and reads a single Furyfile to download packages you need. Fury distribution offers three types of packages:

- **Bases** : Sets of Kustomize bases to help deploying basic components in Kubernetes 
- **Modules**: Terraform modules to deploy kubernetes infrastructure and it’s dependencies
- **Roles**: Ansible roles for deploying, configuring and managing a Kubernetes infrastructure

### Furyfile

Furyfile is a simple YAML formatted file where you list which packages(and versions) you want to have. You can omit a type if you don't need any of its packages. An example Furyfile with packages listed would be like following:

```
roles:
  - name: kube-node
    version: master
  - name: kube-single-master
    version: master

modules:
  - name: aws-single-master
    version: master
  - name: aws-ark
    version: master

bases:
  - name: monitoring/prometheus-operator
    version: master
  - name: monitoring/prometheus-operated
    version: master
  - name: logging
    version: master
```

You can get all packages in a group by using group name (like `logging`) or single packages under a group (like `monitoring-prometheus-operator`).

### Install 

You can get the latest release with:

```
wget https://s3.wasabisys.com/sighup-releases/linux/latest/furyctl \
&& chmod +x furyctl \
&& mv furyctl /usr/local/bin
```

Available endpoints are built as follow:

`https://s3.wasabisys.com/sighup-releases/{arch}/{version}/furyctl`

Supported architectures are (64 bit):
- `linux`
- `darwin`

Current availability versions are: 
- `v0.1.0`
- `latest`


### Usage

Once you installed `furyctl` binary you can see available commands with:


```
$ furyctl --help

A command line tool to manage cluster deployment with kubernetes

Usage:
  furyctl [command]

Available Commands:
  help         Help about any command
  install      Download dependencies specified in Furyfile.yml
  printDefault Prints a basic Furyfile used to generate an INFRA project
  version      Prints the client version information

Flags:
  -h, --help     help for furyctl
  -t, --toggle   Help message for toggle

Use "furyctl [command] --help" for more information about a command.
```

To install packages, from directory where your Furyfile is located run the following command: 

```bash
$ furyctl install

2019/02/04 17:46:07 ----
2019/02/04 17:46:07 SRC:  git@github.com:sighup-io/fury-kubernetes-monitoring//katalog/prometheus-operator?ref=master
2019/02/04 17:46:07 DST:  vendor/katalog/monitoring/prometheus-operator
2019/02/04 17:46:07 ----
2019/02/04 17:46:07 SRC:  git@github.com:sighup-io/fury-kubernetes-monitoring//katalog/prometheus-operator?ref=master
2019/02/04 17:46:07 DST:  vendor/katalog/monitoring/prometheus-operator
...
```   
You will find your packages under `vendor/{roles,modules,katalog}` directories created where you called `furyctl`.


You can get `furyctl` version with command:

```bash
$ furyctl version

Furyctl version  0.1.0
```
