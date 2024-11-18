#!/usr/bin/env sh

set -e

kubectlbin="{{ .paths.kubectl }}"

# Delete old resources after Ingress migration to v3
{{- if eq .spec.distribution.modules.ingress.nginx.type "single" }}
$kubectlbin delete --ignore-not-found=true configmap nginx-configuration -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service ingress-nginx-admission -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service ingress-nginx-metrics -n ingress-nginx
$kubectlbin delete --ignore-not-found=true daemonset.apps nginx-ingress-controller -n ingress-nginx
$kubectlbin delete --ignore-not-found=true certificate.cert-manager.io ingress-nginx-ca -n ingress-nginx
$kubectlbin delete --ignore-not-found=true certificate.cert-manager.io ingress-nginx-tls -n ingress-nginx
$kubectlbin delete --ignore-not-found=true issuer.cert-manager.io ingress-nginx-ca -n ingress-nginx
$kubectlbin delete --ignore-not-found=true issuer.cert-manager.io ingress-nginx-selfsign -n ingress-nginx
$kubectlbin delete --ignore-not-found=true prometheusrule.monitoring.coreos.com ingress-nginx-k8s-rules -n ingress-nginx
$kubectlbin delete --ignore-not-found=true servicemonitor.monitoring.coreos.com ingress-nginx -n ingress-nginx
{{- end }}

{{- if eq .spec.distribution.modules.ingress.nginx.type "dual" }}
$kubectlbin delete --ignore-not-found=true configmap nginx-configuration-external -n ingress-nginx
$kubectlbin delete --ignore-not-found=true configmap nginx-configuration-internal -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service ingress-nginx-admission-external -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service ingress-nginx-admission-internal -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service ingress-nginx-metrics -n ingress-nginx
$kubectlbin delete --ignore-not-found=true daemonset.apps nginx-ingress-controller-external -n ingress-nginx
$kubectlbin delete --ignore-not-found=true daemonset.apps nginx-ingress-controller-internal -n ingress-nginx
$kubectlbin delete --ignore-not-found=true certificate.cert-manager.io ingress-nginx-ca -n ingress-nginx
$kubectlbin delete --ignore-not-found=true certificate.cert-manager.io ingress-nginx-tls-external -n ingress-nginx
$kubectlbin delete --ignore-not-found=true certificate.cert-manager.io ingress-nginx-tls-internal -n ingress-nginx
$kubectlbin delete --ignore-not-found=true issuer.cert-manager.io ingress-nginx-ca -n ingress-nginx
$kubectlbin delete --ignore-not-found=true issuer.cert-manager.io ingress-nginx-selfsign -n ingress-nginx
$kubectlbin delete --ignore-not-found=true prometheusrule.monitoring.coreos.com ingress-nginx-k8s-rules -n ingress-nginx
$kubectlbin delete --ignore-not-found=true servicemonitor.monitoring.coreos.com ingress-nginx -n ingress-nginx
{{- end }}