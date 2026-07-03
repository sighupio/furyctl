#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

## etcd upgrade: stage the etcd sysext + renew certificates, serial:1 (quorum preserved).
{{ .paths.ansiblePlaybook }} upgrade-etcd.yaml --become

## control-plane upgrade: refresh the kubeadm CLI + kubeadm upgrade apply before the reboot, serial:1.
{{ .paths.ansiblePlaybook }} upgrade-control-plane.yml --become

{{- if ne .upgrade.skipNodesUpgrade true }}
## worker upgrade: stage -> drain -> reboot -> kubeadm upgrade node -> uncordon, serial default 1 (configurable).
{{ .paths.ansiblePlaybook }} upgrade-worker-nodes.yml --become
{{- end }}
