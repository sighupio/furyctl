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

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete --ignore-not-found=true validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}

# Upgrade Kyverno steps
{{- if eq .spec.distribution.modules.policy.type "kyverno" }}
echo "scaling down kyverno and deleting webhooks"
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-scale-to-zero.yaml
wait_for_job kyverno kyverno-scale-to-zero 60 5
{{- end }}


echo "removing old eks snapshot-controller"

# TODO check if this is enough
$kubectlbin delete --ignore-not-found=true deployment snapshot-controller -n kube-system
$kubectlbin delete --ignore-not-found=true serviceaccount snapshot-controller -n kube-system
$kubectlbin delete --ignore-not-found=true role snapshot-controller-leaderelection -n kube-system
$kubectlbin delete --ignore-not-found=true clusterrole snapshot-controller-runner -n kube-system
$kubectlbin delete --ignore-not-found=true rolebinding snapshot-controller-leaderelection -n kube-system
$kubectlbin delete --ignore-not-found=true clusterrolebinding snapshot-controller-role
# Skipping CRDs deletion, to not cause deletion of the existing CRs objects in the cluster

