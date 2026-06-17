# furyctl release v0.34.2

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

- [[#658](https://github.com/sighupio/furyctl/pull/658)] Introduced new SIGHUP Distribution `Immutable` kind in alpha status, capable of creating SD clusters with Flatcar Container Linux based nodes.
- [[#671](https://github.com/sighupio/furyctl/pull/671)] On-premises (and Immutable) Ansible can now be provided as a self-contained, versioned bundle (no host Ansible required) when the distribution pins `tools.common.ansible.version`. Distributions that do not pin it keep using the system Ansible, so a host Ansible is still required for those versions. `furyctl download dependencies` now downloads and validates only the dependencies each cluster kind needs. When the distribution pins per-architecture `checksums` for a tool (keyed by `<os>-<arch>`), furyctl verifies the downloaded artifact against the pinned SHA-256 and refuses a mismatch. Note for maintainers: new on-premises upgrade scripts that invoke Ansible must use `{{ $.paths.ansiblePlaybook }}` (root context, since the calls live inside `{{ range }}` blocks) instead of a bare `ansible-playbook`, so they use the bundle when pinned (and the system Ansible otherwise).

## Bug fixes 🐞

TBD

## Breaking Changes 💔

TBD
