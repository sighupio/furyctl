#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete --ignore-not-found=true validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}

# Forecastle namespace migration from ingress-nginx to forecastle
{{- if ne .spec.distribution.modules.ingress.nginx.type "none" }}
$kubectlbin delete --ignore-not-found=true deployment forecastle -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service forecastle -n ingress-nginx
$kubectlbin delete --ignore-not-found=true serviceaccount forecastle -n ingress-nginx
$kubectlbin delete --ignore-not-found=true configmap -n ingress-nginx $($kubectlbin get configmap -n ingress-nginx -o name 2>/dev/null | grep forecastle) 2>/dev/null || true
{{- end }}
