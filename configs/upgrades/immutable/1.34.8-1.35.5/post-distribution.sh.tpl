#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

# Post-distribution upgrade hook — immutable 1.34.8 -> 1.35.5 (alpha).
#
# No distribution-module migration is required AFTER re-applying the modules for
# this transition. If a future {from}-{to} needs to clean up or transform module
# state once the new manifests are applied (delete superseded resources, migrate a
# CRD, drop a removed component), add the steps here, mirroring on-prem's
# configs/upgrades/onpremises/<from>-<to>/post-distribution.sh.tpl.
#
# On-prem pattern (delete resources superseded by the new version):
#   "$kubectlbin" delete <resource> -n <namespace> --ignore-not-found

echo "immutable upgrade: no post-distribution migration required for 1.34.8 -> 1.35.5 (kubectl: ${kubectlbin})"
