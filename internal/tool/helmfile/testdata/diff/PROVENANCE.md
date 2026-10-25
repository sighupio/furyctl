# Provenance

The `helmfile-diff-simple-*` files are real `helmfile diff --output simple --no-color --suppress-secrets` outputs
(`.stdout`, `.stderr`, `.exitcode`) recorded on a lab cluster with helmfile 1.2.3, helm-diff 3.14.1 and helm 3.22.0 on
2026-10-15. `helmfile.yaml` is the helmfile that produced them. The home folder of the lab machine is replaced by
`/opt/user` in the helm binary path, and `helmfile.yaml` has the license header added by `mise run license-check`.

The files named `synthetic-*` are written by hand to cover formats the lab did not record: the helm-diff 3.8.1
format (distributions up to v1.32), suppressed changes, ownership changes, invalid plans and names with `, `.
