#!/usr/bin/env sh

set -e  
    
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
