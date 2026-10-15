# Golden of `furyctl diff --config-only`

`config-only.golden` is the standard output and the exit code, and `config-only.log.golden` the log lines, of
`furyctl diff` as built at c9527433 (before the resource comparison), run by `runDiff` in
`cmd/diff_harness_test.go` against a fake stored configuration that differs in
`.spec.distribution.modules.ingress.nginx.type: single -> dual`. The files were recorded from that build.
