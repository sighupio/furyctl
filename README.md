<!-- markdownlint-disable MD033 -->
<h1 align="center">
  <!-- Using a temporary PNG until we get the updated SVG -->
  <!-- <img src="docs/assets/furyctl-logo.svg" width="200px" alt="furyctl logo" /> -->
  <img src="docs/assets/furyctl-temporary.png" width="200px" alt="furyctl logo" />

<p>The Swiss Army Knife<br/>for the SIGHUP Distribution</p>

[![Build Status](https://ci.sighup.io/api/badges/sighupio/furyctl/status.svg?ref=refs/heads/main)](https://ci.sighup.io/sighupio/furyctl)
![Release](https://img.shields.io/badge/furyctl-v0.32.0-blue)
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
> Starting from v0.25.0, the next generation of `furyctl` has been officially released. Previous versions, furyctl <= 0.11, are considered legacy and will only receive bug fixes. It will be maintained under the v0.11 branch.
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
mise use furyctl@0.32.0
```

Check that everything is working correctly with `furyctl version`:

```bash
$ furyctl version
...
goVersion: go1.23
osArch: amd64
version: 0.32.0
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
version: 0.32.0
```

### Installing from source

Prerequisites:

- `make >= 4.1`
- `go >= 1.23`
- `goreleaser >= 2.3`

> You can install `goreleaser` with the following command once you have Go in your system:
>
> ```bash
> go install github.com/goreleaser/goreleaser/v2@v2.3.2
> ```

Once you've ensured the above dependencies are installed, you can proceed with the installation.

1. Clone the repository:

   ```bash
   git clone git@github.com:sighupio/furyctl.git
   # cd into the cloned repository
   cd furyctl
   ```

2. Build the binaries by running the following command:

   ```bash
   go build .
   ```

3. You will find the binaries for your current architecture inside the current folder:

   ```bash
   $ ls furyctl
   furyctl
   ```

4. Check that the binary is working as expected:

   ```bash
   $ ./furyctl version
   buildTime: 2024-10-08T07:46:28Z
   gitCommit: 217cdcc8bf075fccfdb11c41ccc6bb317ec704bc
   goVersion: go1.23.2
   osArch: arm64
   version: 0.30.1
   ```

5. (optional) move the binary to your `bin` folder, in macOS:

   ```bash
   sudo mv ./furyctl /usr/local/bin/furyctl
   ```

## Usage

For basic and advanced usage instructions, please refer to furyctl's [official documentation](https://docs.sighup.io/furyctl/) and the [SIGHUP Distribution getting started guides](https://docs.sighup.io/docs/getting-started/).


<!-- </SD-DOCS> -->
<!-- <FOOTER> -->

### Test classes

There are four kinds of tests: unit, integration, e2e, and expensive.

Each of them covers specific use cases depending on the speed, cost, and dependencies at play in a given scenario.
Anything that uses i/o should be marked as integration, with the only expectation of local files and folders: any test
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
