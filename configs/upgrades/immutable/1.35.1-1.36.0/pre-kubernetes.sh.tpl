#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

{{- if index .spec "kubernetes" }}

## etcd upgrade: stage the etcd sysext + renew certificates, serial:1 (quorum preserved).
ansible-playbook upgrade-etcd.yaml --become

## control-plane upgrade: refresh the kubeadm CLI + kubeadm upgrade apply before the reboot, serial:1.
ansible-playbook upgrade-control-plane.yml --become

{{- if ne .upgrade.skipNodesUpgrade true }}
## worker upgrade: drain -> stage -> reboot -> kubeadm upgrade node -> uncordon, serial:1.
ansible-playbook upgrade-worker-nodes.yml --become
{{- end }}

{{- end }}
