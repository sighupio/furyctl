#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

# Pre-distribution upgrade hook — immutable 1.34.8 -> 1.35.5 (alpha).
#
# No distribution-module migration is required BEFORE re-applying the modules for
# this transition. If a future {from}-{to} needs to transform or remove module
# state before the new manifests apply (CRD rename, resource move, backend switch),
# add the steps here, mirroring on-prem's
# configs/upgrades/onpremises/<from>-<to>/pre-distribution.sh.tpl.
#
# On-prem pattern (remove stale resources so the new manifests apply clean):
#   "$kubectlbin" delete <resource> -n <namespace> --ignore-not-found

echo "immutable upgrade: no pre-distribution migration required for 1.34.8 -> 1.35.5 (kubectl: ${kubectlbin})"
