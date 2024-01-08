#!/usr/bin/env sh

set -e

kubectlbin="{{ .paths.kubectl }}"

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}

{{ if and (not .spec.distribution.modules.logging.overrides.ingresses.minio.disableAuth) (eq .spec.distribution.modules.auth.provider.type "sso") }}
$kubectlbin delete ingress --ignore-not-found=true --namespace=pomerium minio
{{- end }}

{{- if ne .spec.distribution.modules.dr.type "none" }}
kubectl delete --ignore-not-found=true daemonset -n kube-system node-agent
{{- end }}