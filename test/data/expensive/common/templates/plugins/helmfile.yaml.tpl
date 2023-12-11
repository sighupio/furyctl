{{ if (index .spec "plugins") -}}

{{ if (index .spec.plugins "helm") -}}
{{ if (index .spec.plugins.helm "repositories") -}}
repositories:
{{- toYaml .spec.plugins.helm.repositories | nindent 2 }}
{{- end }}
{{- end }}

{{ if and (index .spec.plugins "helm") (index .spec.plugins.helm "releases")  -}}
releases:
{{- if (and (index .spec.plugins "helm") (index .spec.plugins.helm "releases")) -}}
{{- toYaml .spec.plugins.helm.releases | nindent 2 }}
{{- end -}}
{{- end }}

{{- end }}

helmBinary: {{ .paths.helm }}

helmDefaults:
  args:
    {{- if and (index .spec "kubeconfig") (.paths.kubeconfig) -}}
    - --kubeconfig={{ .paths.kubeconfig }}
    {{- end }}
