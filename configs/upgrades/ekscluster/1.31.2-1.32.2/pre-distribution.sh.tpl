#!/usr/bin/env sh

set -e  

kubectlbin="{{ .paths.kubectl }}"

# Remove some validating webhooks during the upgrade
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
$kubectlbin delete --ignore-not-found=true validatingwebhookconfiguration gatekeeper-validating-webhook-configuration
{{- end }}

# Forecastle namespace migration from ingress-nginx to forecastle
{{- if ne .spec.distribution.modules.ingress.nginx.type "none" }}
$kubectlbin delete --ignore-not-found=true deployment forecastle -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service forecastle -n ingress-nginx
$kubectlbin delete --ignore-not-found=true serviceaccount forecastle -n ingress-nginx
$kubectlbin delete --ignore-not-found=true configmap -n ingress-nginx $($kubectlbin get configmap -n ingress-nginx -o name 2>/dev/null | grep forecastle) 2>/dev/null || true
{{- end }}

# External-DNS namespace migration from ingress-nginx to external-dns
{{- if ne .spec.distribution.modules.ingress.nginx.type "none" }}
$kubectlbin delete --ignore-not-found=true deployment external-dns-public -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service external-dns-metrics-public -n ingress-nginx
$kubectlbin delete --ignore-not-found=true serviceaccount external-dns-public -n ingress-nginx
$kubectlbin delete --ignore-not-found=true deployment external-dns-private -n ingress-nginx
$kubectlbin delete --ignore-not-found=true service external-dns-metrics-private -n ingress-nginx
$kubectlbin delete --ignore-not-found=true serviceaccount external-dns-private -n ingress-nginx
{{- end }}

# Backup Terraform states before introducing OpenTofu
{{- $stateConfig := dict }}
{{- if index .spec.toolsConfiguration "opentofu" }}
  {{- $stateConfig = .spec.toolsConfiguration.opentofu.state.s3 }}
{{- else }}
  {{- $stateConfig = .spec.toolsConfiguration.terraform.state.s3 }}
{{- end }}

s3bucket="{{ $stateConfig.bucketName }}"  
s3keyprefix="{{ $stateConfig.keyPrefix }}"  
s3region="{{ $stateConfig.region }}"  
timestamp=$(date +%s)  

echo "Backing up distribution terraform state to S3..."  

# Upload to S3 with .bkp extension  
aws s3 cp s3://${s3bucket}/${s3keyprefix}/distribution.json s3://${s3bucket}/${s3keyprefix}/distribution.${timestamp}.bkp --region ${s3region}  

echo "Distribution state backed up to s3://${s3bucket}/${s3keyprefix}/distribution.${timestamp}.bkp"
