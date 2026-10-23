# furyctl release vTBD

Welcome to the latest release of `furyctl` maintained by SIGHUP by ReeVo team.

## New features 🌟

### All kinds

- [[#812](https://github.com/sighupio/furyctl/pull/812)] `apply`, `delete cluster`, `serve` and `renew` now report their status to the terminal: the phase, the progress, a prompt that waits for an answer, and the result. A terminal with OSC 7501 support (for example Rex) shows the status on the tab, and the result stays after furyctl stops. A terminal with OSC 9;4 support (for example iTerm2, Ghostty, Windows Terminal, the VS Code terminal) shows a progress bar. Other terminals ignore the sequences, and furyctl sends none with `--no-tty` or when the output is not a terminal.
