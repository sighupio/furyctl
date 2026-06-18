# furyctl release v0.34.2

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

- [[#658](https://github.com/sighupio/furyctl/pull/658)] Introduced new SIGHUP Distribution `Immutable` kind in alpha status, capable of creating SD clusters with Flatcar Container Linux based nodes.
- [[#672](https://github.com/sighupio/furyctl/pull/672)] `furyctl` now downloads and validates only the dependencies each cluster kind actually needs (provider tool sections, the single installer for the kind, and the provider modules). Backward compatible with older distributions: tools pinned under `tools.common` (and the legacy `terraform`) are still resolved, with the provider section (`tools.eks`) taking precedence. Requires `fury-distribution v1.34.2-rc.3`.

## Bug fixes 🐞

TBD

## Breaking Changes 💔

TBD
