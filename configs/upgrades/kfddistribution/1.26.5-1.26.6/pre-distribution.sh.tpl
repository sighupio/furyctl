#!/usr/bin/env sh

set -e

kubectlbin="{{ .paths.kubectl }}"

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete --ignore-not-found=true validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}

# Always remove cerebro, it was removed

$kubectlbin delete --ignore-not-found=true deployment cerebro -n logging
$kubectlbin delete --ignore-not-found=true ingress cerebro -n logging
$kubectlbin delete --ignore-not-found=true ingress cerebro -n pomerium
$kubectlbin delete --ignore-not-found=true svc cerebro -n logging

# Always remove node agent, immutable fields are changed and the resource must be removed and recreated

$kubectlbin delete --ignore-not-found=true daemonset node-agent -n kube-system