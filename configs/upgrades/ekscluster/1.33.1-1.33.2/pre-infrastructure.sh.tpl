#!/usr/bin/env sh

set -e  
  
terraformbin="{{ .paths.terraform }}"  
s3bucket="{{ .spec.toolsConfiguration.opentofu.state.s3.bucketName }}"  
s3keyprefix="{{ .spec.toolsConfiguration.opentofu.state.s3.keyPrefix }}"  
s3region="{{ .spec.toolsConfiguration.opentofu.state.s3.region }}"  
timestamp=$(date +%s)  
  

echo "Backing up infrastructure terraform state to S3..."  
  
# Pull the current state from remote backend 
$terraformbin -chdir=terraform state pull > /tmp/infrastructure-state-${timestamp}.json  
  
# Upload to S3 with .bkp extension  
aws s3 cp /tmp/infrastructure-state-${timestamp}.json s3://${s3bucket}/${s3keyprefix}/infrastructure.${timestamp}.bkp --region ${s3region}  
  
# Clean up local temp file  
rm /tmp/infrastructure-state-${timestamp}.json  
  
echo "Infrastructure state backed up to s3://${s3bucket}/${s3keyprefix}/infrastructure.${timestamp}.bkp"
