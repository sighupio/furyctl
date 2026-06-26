#!/usr/bin/env sh

set -eu

kubectlbin="{{ .paths.kubectl }}"

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete --ignore-not-found=true validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}

# Migrate Tempo memcached resources for tracing module
{{- if eq .spec.distribution.modules.tracing.type "tempo" }}
$kubectlbin -n tracing delete --ignore-not-found=true service --selector 'app.kubernetes.io/instance=tempo-distributed,app.kubernetes.io/component in (memcached,memcached-bloom,memcached-parquet-footer,memcached-frontend-search)'
$kubectlbin -n tracing delete --ignore-not-found=true statefulset --selector 'app.kubernetes.io/instance=tempo-distributed,app.kubernetes.io/component in (memcached,memcached-bloom,memcached-parquet-footer,memcached-frontend-search)' --cascade=orphan
{{- end }}