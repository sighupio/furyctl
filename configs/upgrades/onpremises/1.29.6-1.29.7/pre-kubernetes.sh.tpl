#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

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

{{- if index .spec "kubernetes" }}
# Update only etcd, not kubernetes.
{{- if index .spec.kubernetes "etcd" }}
## etcd upgrades on dedicated nodes - only one at a time
{{- range $h := .spec.kubernetes.etcd.hosts }}
ansible-playbook 54.upgrade-etcd.yaml --limit "{{ $h.name }}" --become
{{- end }}
{{ else }}
## etcd upgrades on control plane nodes - only one at a time
{{- range $h := .spec.kubernetes.masters.hosts }}
ansible-playbook 54.upgrade-etcd.yaml --limit "{{ $h.name }}" --become
{{- end }}
{{- end }}
{{- end }}
