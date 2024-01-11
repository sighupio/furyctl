[default]
{{- if ne .spec.distribution.modules.dr.type "none" }}
aws_access_key_id = {{ .spec.distribution.modules.dr.velero.externalEndpoint.accessKeyId }}
aws_secret_access_key = {{ .spec.distribution.modules.dr.velero.externalEndpoint.secretAccessKey }}
{{- end }}
