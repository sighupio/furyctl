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

# Delete Cilium Hubble mTLS certs so they are recreated to force a renew
{{- if eq .spec.distribution.modules.networking.type "cilium" }}
$kubectlbin delete --ignore-not-found=true certificate -n kube-system hubble-relay-client-certs hubble-server-certs
$kubectlbin delete --ignore-not-found=true secrets -n kube-system hubble-relay-client-certs hubble-server-certs
{{- end }}

# Upgrade Kyverno steps
{{- if eq .spec.distribution.modules.policy.type "kyverno" }}
echo "scaling down kyverno and deleting webhooks"
$kubectlbin apply --server-side -f $vendorPath/modules/opa/katalog/kyverno/upgrade-paths/v1.12.6-v1.13.4/kyverno-scale-to-zero.yaml
wait_for_job kyverno kyverno-scale-to-zero 60 5
{{- end }}


# Scale Loki's Ingester if needed and apply new flags for flushing on shutdown
# We need to do this here because we want to ensure that the ingester is properly
# setup for not losing logs before we start the nodes draining process.
{{- if eq .spec.distribution.modules.logging.type "loki" }}
ingester_replicas=$($kubectlbin get statefulsets.apps -n logging loki-distributed-ingester -o jsonpath={.status.currentReplicas})
if [ "${ingester_replicas}" -lt "2" ]; then
  echo "Scaling up Loki Ingester to 2 replicas..."
  $kubectlbin scale sts -n logging loki-distributed-ingester --replicas=2
  echo "Waiting for Loki Ingester 2 replicas to be available..."
  $kubectlbin wait -n logging statefulset/loki-distributed-ingester --timeout=5m --for=jsonpath='{.status.availableReplicas}'=2
fi

echo "Patching Loki Ingester to flush on shutdown..."
$kubectlbin patch statefulset loki-distributed-ingester -n logging --type='json' -p="[{\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/args\",\"value\":[\"-config.file=/etc/loki/config/config.yaml\",\"-ingester.flush-on-shutdown=true\",\"-log.level=debug\",\"-target=ingester\"]}]"
$kubectlbin rollout status -n logging statefulset/loki-distributed-ingester
{{- end }}
