# Kubernetes Fury Distribution universal upgrade guide

This guide describes the steps to follow to upgrade the Kubernetes Fury Distribution from one versions to the next.

If you are running a custom set of modules, or different versions than the ones included with each release of KFD, please refer to each module's release notes.

> ⛔️ **IMPORTANT**
> we strongly recommend reading the whole guide before starting the upgrade process to identify possible blockers.

## Upgrade procedure

Check the available automatic upgrade paths [here](https://github.com/sighupio/furyctl/tree/main/configs/upgrades). These are the tested and suggested upgrade paths to be used.

### Run the upgrade

Change `.spec.distributionVersion` on your `furyctl.yaml` file with the new `vX.X.X` version.

Validate the schema using:

```bash
furyctl validate config
```

Apply the new configuration on the cluster with:

```bash
furyctl apply --upgrade
```

#### Additional useful flags when upgrading

##### OnPremises

In the OnPremises provider, during the upgrade, you can use the `--skip-nodes-upgrade` flag to skip the actual upgrade of the worker nodes and only do the upgrade of the masters.

In a second moment, you can run for each worker, the command

```bash
furyctl apply --upgrade-node <nodename>
```

where `nodename` is the name in the furyctl.yaml file

##### Upgrade fails during a phase

You can run the command

```bash
furyctl apply --upgrade
```

and furyctl will start from the last successful phase. If you want to start from a different phase, you can use the flag `--start-from` like this:

```bash
furyctl apply --upgrade --start-from pre-distribution
```

you can find all the available parameters with the `furyctl apply --upgrade --help` command.

## Manual upgrade procedure

To upgrade your cluster to the next version manually, follow the release notes for each module and installer.