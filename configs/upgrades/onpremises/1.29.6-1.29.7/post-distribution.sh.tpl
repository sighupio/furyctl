#!/usr/bin/env sh

set -e

vendorPath="{{ .paths.vendorPath }}"
kubectlbin="{{ .paths.kubectl }}"
yqbin="{{ .paths.yq }}"

# Upgrade Kyverno steps
{{- if eq .spec.distribution.modules.policy.type "kyverno" }}
echo "resource migration"
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-migrate-resources-sa.yaml
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-migrate-resources-role.yaml
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-migrate-resources-binding.yaml
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-migrate-resources-job.yaml
echo "cleaning policy reports"
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-clean-reports.yaml
{{- end }}

# Logging v5 / Loki cleanup
{{- if eq .spec.distribution.modules.logging.type "loki" }}
echo "cleaning logging resources..."
# Delete `loki-distributed-querier` statefulset that has been migrated to a deployment
$kubectlbin delete statefulset --namespace logging loki-distributed-querier --ignore-not-found
# Delete `loki-distributed-compactor` deployment that has been migrated to a statefulset
$kubectlbin delete deployment --namespace logging loki-distributed-compactor --ignore-not-found
# Delete old `loki-distributed-*` configuration secrets (they have been replaced by `loki-*`)
$kubectlbin get secrets --namespace logging --output yaml | $yqbin eval '.items[] | select(.metadata.name | test("^loki-distributed")) | .metadata.name' - | xargs -I {} $kubectlbin delete secret --namespace logging --ignore-not-found {}
echo "finished cleaning logging resources"
{{- end }}
