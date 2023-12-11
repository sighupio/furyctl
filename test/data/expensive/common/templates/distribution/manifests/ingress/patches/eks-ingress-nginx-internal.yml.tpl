# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

{{- if eq .spec.distribution.common.provider.type "eks" }}
{{ if eq .spec.distribution.modules.ingress.nginx.type "dual" -}}
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "external"
    service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: "instance"
    service.beta.kubernetes.io/aws-load-balancer-proxy-protocol: "*"
  name: ingress-nginx-internal
  namespace: ingress-nginx
spec:
  type: LoadBalancer
  externalTrafficPolicy: Cluster
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-configuration-internal
  namespace: ingress-nginx
data:
  use-proxy-protocol: "true"
{{- end }}
{{- end }}
