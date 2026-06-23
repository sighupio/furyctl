#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

{{- if index .spec "kubernetes" }}

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

## master upgrades - only one at a time
{{- range $h := .spec.kubernetes.masters.hosts }}
ansible-playbook 55.upgrade-control-plane.yml --limit "{{ $h.name }}" --become
{{- end }}

{{- if ne .upgrade.skipNodesUpgrade true }}
{{- range $n := .spec.kubernetes.nodes }}
    {{- range $h := $n.hosts }}
ansible-playbook 56.upgrade-worker-nodes.yml --limit "{{ $h.name }}"
    {{- end }}
{{- end }}
{{- end }}

{{- end }}
