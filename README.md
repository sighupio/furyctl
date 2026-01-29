<!-- markdownlint-disable MD033 -->
<h1 align="center">
  <img src="docs/assets/furyctl-temporary.png" width="200px" alt="furyctl logo" />

   <p>The Swiss Army Knife<br/>for the SIGHUP Distribution</p>

   [![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg?ref=refs/heads/main)](https://ci.sighup.io/sighupio/furyctl)
   ![Release](https://img.shields.io/badge/furyctl-v0.33.3-blue)
   ![Slack](https://img.shields.io/badge/slack-@kubernetes/fury-yellow.svg?logo=slack)
   ![License](https://img.shields.io/github/license/sighupio/furyctl)
   [![Go Report Card](https://goreportcard.com/badge/github.com/sighupio/furyctl)](https://goreportcard.com/report/github.com/sighupio/furyctl)

</h1>
<!-- markdownlint-eable MD033 -->

<!-- <SD-DOCS> -->

## What is furyctl?

`furyctl` is the command line companion for the SIGHUP Distribution (SD) to manage the **full lifecycle** of your SD Kubernetes clusters.
<br/>

> [!TIP]
> Learn more about the SIGHUP Distribution in the [documentation site](https://docs.sighup.io).
<!-- spacer -->
> [!NOTE]
> Starting from v0.25.0, the next generation of `furyctl` has been officially released. Previous versions (<= 0.11), are considered legacy and will only receive bug fixes. It will be maintained under the `release-v0.11` branch.
>
> If you're looking for the old documentation for furyctl legacy, you can find it [here](https://github.com/sighupio/furyctl/blob/release-v0.11/README.md).

### Available providers

- `EKSCluster`: Provides comprehensive lifecycle management for a SIGHUP Distribution Kubernetes cluster based on EKS from AWS. It handles the installation of the VPC, VPN, EKS using the installers, and deploys the Distribution onto the EKS cluster.
- `KFDDistribution`: Dedicated provider for the distribution, which installs the SIGHUP Distribution (modules only) on an existing Kubernetes cluster.
- `OnPremises`: Provider to manage the full lifecycle of a SIGHUP Distribution cluster on Virtual Machines.

## Support & Compatibility 🪢

Check the [compatibility matrix][compatibility-matrix] for information about `furyctl` and `SD` versions compatibility.

## Installation

### Installing from binaries

You can find `furyctl` binaries on the [Releases page](https://github.com/sighupio/furyctl/releases).

To download the latest release, run:

```bash
curl -L "https://github.com/sighupio/furyctl/releases/latest/download/furyctl-$(uname -s)-amd64.tar.gz" -o /tmp/furyctl.tar.gz && tar xfz /tmp/furyctl.tar.gz -C /tmp
chmod +x /tmp/furyctl
sudo mv /tmp/furyctl /usr/local/bin/furyctl
```

Alternatively, you can install `furyctl` using `mise` or the `asdf` plugin.

### Installing with [mise](https://mise.jdx.dev/)

```bash
mise use furyctl@0.33.3
```

Check that everything is working correctly with `furyctl version`:

```bash
$ furyctl version
...
goVersion: go1.23
osArch: arm64
version: 0.33.3
```

### Installing with [asdf](https://github.com/asdf-vm/asdf)

Add furyctl asdf plugin:

```bash
asdf plugin add furyctl
```

Check that everything is working correctly with `furyctl version`:

```bash
$ furyctl version
...
goVersion: go1.23
osArch: amd64
version: 0.33.3
```

## Development

For development setup, building from source, and contributing guidelines, see [DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Usage

For basic and advanced usage instructions, please refer to furyctl's [official documentation](https://docs.sighup.io/furyctl/) and the [SIGHUP Distribution getting started guides](https://docs.sighup.io/docs/getting-started/).

<!-- </SD-DOCS> -->
<!-- <FOOTER> -->

## Reporting Issues

In case you experience any problems with `furyctl` itslef, please [open a new issue](https://github.com/sighupio/furyctl/issues/new/choose) on GitHub. If the issue is related to the SIGHUP Distribution, please open the issue in [its repository](https://github.com/sighupio/distribution) instead.

## License

This software is open-source and it's released under the following [LICENSE](LICENSE).

<!-- </FOOTER> -->

[compatibility-matrix]: https://github.com/sighupio/furyctl/blob/main/docs/COMPATIBILITY_MATRIX.md
