/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

output "cluster_endpoint" {
  description = "The endpoint for your Kubernetes API server"
  value       = module.fury.cluster_endpoint
}

output "cluster_certificate_authority" {
  description = "The base64 encoded certificate data required to communicate with your cluster. Add this to the certificate-authority-data section of the kubeconfig file for your cluster"
  value       = module.fury.cluster_certificate_authority
}

output "operator_ssh_user" {
  description = "SSH user to access cluster nodes with ssh_public_key"
  value       = module.fury.operator_ssh_user
}

output "eks_cluster_oidc_issuer_url" {
  description = "The URL on the EKS cluster OIDC Issuer"
  value       = module.fury.eks_cluster_oidc_issuer_url
}

output "eks_worker_iam_role_name" {
  description = "Default IAM role name for EKS worker groups"
  value       = module.fury.eks_worker_iam_role_name
}

output "kubeconfig" {
  sensitive = true
  value     = <<EOT
apiVersion: v1
clusters:
- cluster:
    server: ${module.fury.cluster_endpoint}
    certificate-authority-data: ${module.fury.cluster_certificate_authority}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
kind: Config
preferences: {}
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: aws
      args:
        - "eks"
        - "get-token"
        - "--cluster-name"
        - "${var.cluster_name}"
EOT
}
