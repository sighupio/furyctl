# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
{{- if ne .spec.distribution.modules.dr.type "none" }}

{{- if eq .spec.distribution.common.provider.type "eks" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/dr/katalog/velero/velero-aws" }}
{{- else if eq .spec.distribution.common.provider.type "none" }}

{{- if eq .spec.distribution.modules.dr.velero.backend "minio" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/dr/katalog/velero/velero-on-prem" }}
{{- else }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/dr/katalog/velero/velero-aws" }}
  - resources/storageLocation.yaml
{{- end }}


  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/dr/katalog/velero/velero-node-agent" }}

{{- end }}

  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/dr/katalog/velero/velero-schedules" }}

{{- if eq .spec.distribution.common.provider.type "eks" }}
  - resources/eks-velero-backupstoragelocation.yml
  - resources/eks-velero-volumesnapshotlocation.yml
{{- end }}

patchesStrategicMerge:
  - patches/infra-nodes.yml
{{- if eq .spec.distribution.common.provider.type "eks" }}
  - patches/eks-velero.yml
{{- end }}

{{- if eq .spec.distribution.common.provider.type "none" }}
{{- if eq .spec.distribution.modules.dr.velero.backend "externalEndpoint" }}
secretGenerator:
- name: cloud-credentials
  namespace: kube-system
  files:
    - cloud=secrets/cloud-credentials.config
{{- end }}
{{- end }}

{{- end }}
