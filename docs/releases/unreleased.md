# furyctl release vTBD

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

### All kinds

- [[#808](https://github.com/sighupio/furyctl/pull/808)] When a tool that furyctl runs fails, the error now starts with the cause, for example each Ansible host that failed or does not answer, then gives the command and the last lines of its output. furyctl saves the full output to a text file in `.furyctl/logs` and gives its path. Before this release, the error repeated the name of the command and gave its full output, and the cause was difficult to find in it.
- [[#392](https://github.com/sighupio/furyctl/issues/392)] `furyctl diff` now prints a CONFIG table. The table has one row for each changed field, and the rows are grouped by phase (General, Infrastructure, Kubernetes, Distribution, Plugins). The columns are FIELD, CURRENT, NEW and RULE, and a first line gives the number of changes. The order of the rows is the same on every run. A list item shows its name or its host name, not its position.
- [[#392](https://github.com/sighupio/furyctl/issues/392)] The RULE column of `furyctl diff` tells what `furyctl apply` does with each change, from the rules of the distribution: `blocked` (the apply stops), `needs confirmation` (the apply asks before it continues) or `migration` (the apply runs a migration). An empty RULE cell means that no rule matches the change, not that the change has no effect. When the distribution has no rules file for the kind, or the file cannot be read, the table has no verdicts and its last line says why, as `furyctl apply` skips its rule checks in that case.
- [[#392](https://github.com/sighupio/furyctl/issues/392)] On a terminal narrower than the table, `furyctl diff` narrows the CURRENT, NEW and FIELD columns so that the verdict (`✗ blocked`, `! needs confirmation`, `· migration`) is always shown in full; only the explanation of the rule is cut. A file, a pipe or `--no-tty` gets every value in full and no colour. The `NO_COLOR` environment variable turns the colour off.
- [[#392](https://github.com/sighupio/furyctl/issues/392)] `furyctl diff` and the debug log of `furyctl apply` no longer show secret values. A value is printed as `(hidden)` when the name of its field contains `pass`, `secret`, `token`, `key`, `credential` or `authorization`, or when the field is known to hold credentials: Helm `set` values, Dex `bindPW`, Alertmanager webhook URLs, `remoteWrite` headers, custom patches, logging custom outputs, the encryption and cloud configuration, the HAProxy configuration, iPXE commands, node file contents and systemd units. The user and password of a URL are hidden too.

## Breaking Changes 💔

### All kinds

- [[#392](https://github.com/sighupio/furyctl/issues/392)] The output format of `furyctl diff` changed. Before this release the command printed one `.path: old -> new` line for each change. It now prints the CONFIG table, and scripts that parse the old lines must be changed. With no changes it prints the line `CONFIG   no changes`, where before it only wrote a log message. The flags and the exit code are the same.
