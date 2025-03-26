#!/usr/bin/env sh

set -e

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
