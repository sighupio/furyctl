# furyctl release vTBD

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

- [[#741](https://github.com/sighupio/furyctl/pull/741)] Immutable, OnPremises: furyctl now checks the PKI folder from the configuration file before an apply. When the folder or one of its files is absent, the apply stops before it starts the playbooks, and the message names the `furyctl create pki` command to run. Before this release, the apply failed in the middle, inside an Ansible task, with a message that did not say how to correct the fault. `furyctl validate config` does the same check, so a pipeline that validates a configuration now needs the PKI folder on that machine.

## Bug fixes 🐞

TBD

## Breaking Changes 💔

None
