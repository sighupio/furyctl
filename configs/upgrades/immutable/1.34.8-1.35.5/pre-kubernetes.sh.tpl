#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

{{- if index .spec "kubernetes" }}

## etcd upgrade: align the etcd sysext + renew certificates. Group-targeted; the
## playbook runs serial:1 (one node at a time, quorum preserved).
ansible-playbook 54.upgrade-etcd.yaml --become

## control-plane upgrade: sysext (kubernetes/kubeadm) + `kubeadm upgrade apply`
## (apply-before-reboot) + OS stage/reboot. serial:1, one control plane at a time.
ansible-playbook 55.upgrade-control-plane.yml --become

{{- if ne .upgrade.skipNodesUpgrade true }}
## worker upgrade: drain -> sysext + `kubeadm upgrade node` + OS reboot -> uncordon.
## serial:1, one worker at a time.
ansible-playbook 56.upgrade-worker-nodes.yml --become
{{- end }}

{{- end }}
