# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

{{ if .spec.distribution.modules.ingress }}
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - {{ .spec.distribution.common.relativeVendorPath }}/katalog/ingress/cert-manager
{{- if eq .spec.distribution.modules.ingress.nginx.type "dual" }}
  - {{ .spec.distribution.common.relativeVendorPath }}/katalog/ingress/dual-nginx
{{- end }}
{{- if eq .spec.distribution.modules.ingress.nginx.type "single" }}
  - {{ .spec.distribution.common.relativeVendorPath }}/katalog/ingress/nginx
{{- end }}
  - {{ .spec.distribution.common.relativeVendorPath }}/katalog/ingress/forecastle
{{- if .spec.distribution.modules.ingress.certManager.clusterIssuer.notExistingProperty }}
  - resources/cert-manager-clusterissuer.yml
{{- end }}

patchesStrategicMerge:
{{- if .spec.distribution.modules.ingress.certManager.clusterIssuer.notExistingProperty }}
  - patches/cert-manager.yml
{{- end }}
  - patches/infra-nodes.yml
  - patches/ingress-nginx.yml
{{- end }}
