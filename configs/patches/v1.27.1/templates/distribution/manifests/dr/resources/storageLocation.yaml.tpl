# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

---
apiVersion: velero.io/v1
kind: BackupStorageLocation
metadata:
  labels:
    k8s-app: velero
  name: default
  namespace: kube-system
spec:
  config:
    region: custom
    s3ForcePathStyle: "true"
    {{- if ne .spec.distribution.modules.dr.type "none" }}
    s3Url: {{ ternary "http" "https" .spec.distribution.modules.dr.velero.externalEndpoint.insecure }}://{{ .spec.distribution.modules.dr.velero.externalEndpoint.endpoint }}
    {{- end }}
  objectStorage:
    {{- if ne .spec.distribution.modules.dr.type "none" }}
    bucket: {{ .spec.distribution.modules.dr.velero.externalEndpoint.bucketName }}
    {{- end }}
  provider: aws
