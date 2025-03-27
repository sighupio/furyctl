#!/usr/bin/env sh

set -e

wait_for_job() {
  local kubectlbin="{{ .paths.kubectl }}"  
  local namespace="$1"
  local jobname="$2"
  local timeout="$3"
  local retries="$4"
  local retries_count=0
  
  while [ $retries_count -lt "$retries" ]; do
    if $kubectlbin wait --for=condition=complete job/"$jobname" -n "$namespace" --timeout="${timeout}s"; then
      echo "job completed"
      return 0
    fi
    
    echo "timeout"
    
    retries_count=$((retries_count+1))
    
    if [ $retries_count -lt "$retries" ]; then
      echo "retry"
    else
      $kubectlbin logs -n "$namespace" job/"$jobname"
      echo "exit"
      exit 1
    fi
  done
}

vendorPath="{{ .paths.vendorPath }}"
kubectlbin="{{ .paths.kubectl }}"

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