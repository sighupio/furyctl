# furyctl release vTBD

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

TBD

## Bug fixes 🐞

- [[#695](https://github.com/sighupio/furyctl/pull/695)] Immutable: fixed a panic that may occur if the user pressed ENTER to while waiting the nodes to be provisioned and a node sent a status update in the 5 seconds shutdown window.
- [[#697](https://github.com/sighupio/furyctl/pull/697)] EKSCluster: `openvpn` is now validated as a dependency whenever a VPN is configured (not only with `--vpn-auto-connect`), failing fast with a clear error instead of a silent furyagent retry loop; `--vpn-auto-connect` without a configured VPN is now rejected up front.

## Breaking Changes 💔

TBD
