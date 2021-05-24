/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

output "cluster_endpoint" {
  sensitive   = true
  description = "The endpoint for your Kubernetes API server"
  value       = module.fury.cluster_endpoint
}

output "cluster_certificate_authority" {
  sensitive   = true
  description = "The base64 encoded certificate data required to communicate with your cluster. Add this to the certificate-authority-data section of the kubeconfig file for your cluster"
  value       = module.fury.cluster_certificate_authority
}

output "operator_ssh_user" {
  description = "SSH user to access cluster nodes with ssh_public_key"
  value       = module.fury.operator_ssh_user
}

data "google_client_config" "current" {}

output "kubeconfig" {
  sensitive = true
  value     = <<EOT
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ${module.fury.cluster_certificate_authority}
    server: ${module.fury.cluster_endpoint}
  name: ${var.cluster_name}
contexts:
- context:
    cluster: ${var.cluster_name}
    user: operator
  name: ${var.cluster_name}-ctx
current-context: ${var.cluster_name}-ctx
kind: Config
preferences: {}
users:
- name: operator
  user:
    token: ${data.google_client_config.current.access_token}
EOT
}
