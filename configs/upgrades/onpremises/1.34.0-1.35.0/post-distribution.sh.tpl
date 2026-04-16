#!/usr/bin/env sh

set -e

kubectlbin="{{ .paths.kubectl }}"

{{- if .spec | digAny "kubernetes" "advanced" "kubeProxy" "enabled" true }}
  {{- if eq .spec.distribution.modules.networking.type "calico" }}
echo "Restarting daemonset/calico-node to apply nftables dataplane..."
$kubectlbin rollout restart -n calico-system daemonset/calico-node
$kubectlbin rollout status -n calico-system daemonset/calico-node --timeout=300s
echo "calico-node restarted"
  {{- end }}
  {{- if eq .spec.distribution.modules.networking.type "cilium" }}
echo "Restarting daemonset/cilium to apply kube-proxy mode change..."
$kubectlbin rollout restart -n kube-system daemonset/cilium
$kubectlbin rollout status -n kube-system daemonset/cilium --timeout=300s
echo "Cilium restarted"
  {{- end }}
{{- end }}
