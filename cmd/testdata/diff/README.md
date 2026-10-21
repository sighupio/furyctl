# Golden of `furyctl diff --config-only`

`config-only.golden` is the standard output and the exit code, and `config-only.log.golden` the log lines, of
`furyctl diff --config-only`, run by `runDiff` in `cmd/diff_harness_test.go` against a fake stored configuration
that differs in `.spec.distribution.modules.ingress.nginx.type: single -> dual`. The fake distribution has no rules
file for the kind, so the CONFIG table has no RULE verdict and ends with the line that says why. The files were
recorded from the command itself; record them again when the CONFIG table changes on purpose.
