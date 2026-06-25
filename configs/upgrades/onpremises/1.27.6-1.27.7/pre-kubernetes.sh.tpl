#!/usr/bin/env sh

set -e

{{- if index .spec "kubernetes" }}

## launch create playbook on haproxy nodes due to an update on the underlying role
{{ $.paths.ansiblePlaybook }} create-playbook.yaml --tags haproxy

{{- end }}
