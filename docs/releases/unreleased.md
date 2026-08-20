# furyctl release vTBD

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

- [[#741](https://github.com/sighupio/furyctl/pull/741)] Immutable, OnPremises: furyctl now checks the PKI folder from the configuration file before an apply. When the folder or one of its files is absent, the apply stops before it starts the playbooks, and the message names the `furyctl create pki` command to run. Before this release, the apply failed in the middle, inside an Ansible task, with a message that did not say how to correct the fault. `furyctl validate config` does the same check, so a pipeline that validates a configuration now needs the PKI folder on that machine.
- [[#745](https://github.com/sighupio/furyctl/pull/745)] OnPremises and Immutable: the new `furyctl renew kubeconfigs` command renews the kubeconfig file of the admin and the kubeconfig files of the users in `spec.kubernetes.advanced.users.names`. It writes them to the working directory, with the names that `furyctl apply` uses. A list of names renews only some of them, for example `furyctl renew kubeconfigs admin alice`. A user that you add to the configuration file gets a kubeconfig file. It is not necessary to apply the kubernetes phase.

## Bug fixes 🐞

- [[#743](https://github.com/sighupio/furyctl/pull/743)] All kinds: `apply --start-from` now stops when the configuration has changes to a phase before the selected one. Before this fix furyctl skipped those changes and wrote the new configuration to the cluster, so the change was lost and the next `diff` did not show it as pending.
- [[#746](https://github.com/sighupio/furyctl/issues/746)] All kinds: a flag that you give on the command line now has precedence over the same flag in the `flags` section of `furyctl.yaml`. The documented priority is `furyctl.yaml` < environment variable < command line. Before this release the commands `diff`, `create config`, `get cluster-info` and `get upgrade-paths` read the configuration file first, thus the configuration file had the precedence. If you want the value of the configuration file, do not give the flag on the command line.


## Breaking Changes 💔

None

