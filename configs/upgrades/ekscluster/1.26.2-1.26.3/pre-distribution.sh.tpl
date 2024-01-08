#!/usr/bin/env sh

set -e

kubectlbin="{{ .paths.kubectl }}"

$kubectlbin delete --ignore-not-found=true deployment ebs-csi-controller -n kube-system
$kubectlbin delete --ignore-not-found=true daemonset ebs-csi-node -n kube-system
$kubectlbin delete --ignore-not-found=true daemonset ebs-csi-node-windows -n kube-system

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete --ignore-not-found=true validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}
