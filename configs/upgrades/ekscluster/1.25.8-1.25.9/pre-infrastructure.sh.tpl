#!/usr/bin/env sh

set -e

terraformbin="{{ .paths.terraform }}"

{{ $hasVpnEnabled := (
    and
        (and (index .spec "infrastructure") (index .spec.infrastructure "vpn"))
        (
            or
                (not (index .spec.infrastructure.vpn "instances"))
                (gt (index .spec.infrastructure.vpn "instances") 0)
        )
) }}

{{- $bucketName := "" }}
{{- if index .spec.toolsConfiguration "opentofu" }}
  {{- $bucketName = .spec.toolsConfiguration.opentofu.state.s3.bucketName }}
{{- else }}
  {{- $bucketName = .spec.toolsConfiguration.terraform.state.s3.bucketName }}
{{- end }}

{{- if $hasVpnEnabled }}
TF_STATE=$($terraformbin -chdir=terraform state list)

if [ ! $(echo "${TF_STATE}" | grep -F 'module.vpn[0].aws_s3_bucket_ownership_controls.furyagent') ]; then
    $terraformbin -chdir=terraform import \
        module.vpn[0].aws_s3_bucket_ownership_controls.furyagent \
        "{{ $bucketName }}"
fi

if [ ! $(echo "${TF_STATE}" | grep -F 'module.vpn[0].aws_s3_bucket_server_side_encryption_configuration.furyagent') ]; then
    $terraformbin -chdir=terraform import \
        module.vpn[0].aws_s3_bucket_server_side_encryption_configuration.furyagent \
        "{{ $bucketName }}"
fi

if [ ! $(echo "${TF_STATE}" | grep -F 'module.vpn[0].aws_s3_bucket_versioning.furyagent') ]; then
    $terraformbin -chdir=terraform import \
        module.vpn[0].aws_s3_bucket_versioning.furyagent \
        "{{ $bucketName }}"
fi
{{- end -}}
