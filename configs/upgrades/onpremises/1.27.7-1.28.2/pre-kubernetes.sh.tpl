#!/usr/bin/env sh

set -e

{{- if index .spec "kubernetes" }}

## launch create playbook on haproxy nodes due to an update on the underlying role
{{ .paths.ansiblePlaybook }} create-playbook.yaml --tags haproxy

## master upgrades - only one at a time
{{- range $h := .spec.kubernetes.masters.hosts }}
{{ .paths.ansiblePlaybook }} 55.upgrade-control-plane.yml --limit "{{ $h.name }}" --become
{{- end }}

{{- if ne .upgrade.skipNodesUpgrade true }}
{{- range $n := .spec.kubernetes.nodes }}
    {{- range $h := $n.hosts }}
{{ .paths.ansiblePlaybook }} 56.upgrade-worker-nodes.yml --limit "{{ $h.name }}"
    {{- end }}
{{- end }}
{{- end }}

{{- end }}
