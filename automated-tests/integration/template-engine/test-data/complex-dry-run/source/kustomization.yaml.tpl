{{- if .modules.ingress }}
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - {{ .common.relativeVendorPath }}/katalog/ingress/cert-manager
{{- if eq .modules.ingress.nginx.type "dual" }}
  - {{ .common.relativeVendorPath }}/katalog/ingress/dual-nginx
{{- end }}
{{- if eq .modules.ingress.nginx.type "single" }}
  - {{ .common.relativeVendorPath }}/katalog/ingress/nginx
{{- end }}
  - {{ .common.relativeVendorPath }}/katalog/ingress/forecastle
{{- if .modules.ingress.certManager.clusterIssuer.notExistingProperty }}
  - resources/cert-manager-clusterissuer.yml
{{- end }}

patchesStrategicMerge:
{{- if .modules.ingress.certManager.clusterIssuer.notExistingProperty }}
  - patches/cert-manager.yml
{{- end }}
  - patches/infra-nodes.yml
  - patches/ingress-nginx.yml
{{- end }}
