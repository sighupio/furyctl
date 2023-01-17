{{ if eq .spec.distribution.modules.ingress.nginx.type "dual" -}}
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
    service.beta.kubernetes.io/aws-load-balancer-type: "external"
    service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: "instance"
    service.beta.kubernetes.io/aws-load-balancer-proxy-protocol: "*"
  name: ingress-nginx-external
  namespace: ingress-nginx
spec:
  type: LoadBalancer
  externalTrafficPolicy: Cluster
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-configuration-external
  namespace: ingress-nginx
data:
  use-proxy-protocol: "true"
{{- end }}
