# furyctl release vTBD

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

### All kinds

- [[#812](https://github.com/sighupio/furyctl/pull/812)] `apply`, `delete cluster`, `serve` and `renew` now report their status to the terminal: the phase, the progress, a prompt that waits for an answer, and the result. A terminal with OSC 7501 support (for example Rex) shows the status on the tab, and the result stays after furyctl stops. A terminal with OSC 9;4 support (for example iTerm2, Ghostty, Windows Terminal, or the VS Code terminal with `${progress}` in its tab title) shows a progress bar. Other terminals ignore the sequences, and furyctl sends none with `--no-tty` or when the output is not a terminal.

### Immutable

- [[#820](https://github.com/sighupio/furyctl/pull/820)] `get cluster-info` now shows the Flatcar version and the kernel version of the nodes. If the nodes run different versions, for example during an upgrade, the command shows all of them.
- [[#819](https://github.com/sighupio/furyctl/pull/819)] `delete cluster` now works for the Immutable kind, by phase and in the reverse order of `apply`: the plugins, the distribution (`kapp delete -a kfd`) and the plugins that provide a StorageClass, with the delete scripts of the distribution, then the infrastructure, which checks every node and erases the partition table of its install disk. The nodes restart, boot from the network and wait there until `furyctl apply` installs them again. `--phase plugins|distribution|kubernetes|infrastructure` runs one phase, and `kubernetes` deletes nothing. Objects still stuck on a finalizer after 15 minutes are reported, not forced. Without a reachable Kubernetes API, use `--phase infrastructure`. Other disks are kept, and this is not a secure erase. `--dry-run` renders the scripts, checks every node and changes nothing. It needs a distribution version that ships the delete scripts and the delete playbook ([distribution#644](https://github.com/sighupio/distribution/pull/644)).

## Bug fixes 🐞

### All kinds

- [[#817](https://github.com/sighupio/furyctl/pull/817)] A dry run no longer leaves a copy of the configuration file, with its inlined secrets if any, in the temporary folder.
