/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

{{- $deprecateOptionalTfVer := semver "1.3.0" }}

terraform {
  {{ if eq ($deprecateOptionalTfVer | (semver .kubernetes.tfVersion).Compare) -1 -}}
  experiments = [module_variable_optional_attrs]
  {{ end -}}

  backend "s3" {
    bucket = "{{ .terraform.backend.s3.bucketName }}"
    key    = "{{ .terraform.backend.s3.keyPrefix }}/cluster.json"
    region = "{{ .terraform.backend.s3.region }}"

    skip_region_validation = true
  }

  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}

provider "aws" {
  region = "{{ .spec.region }}"
  default_tags {
    tags = {
      {{- range $k, $v := .spec.tags }}
      {{ $k }} = "{{ $v }}"
      {{- end}}
    }
  }
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.fury.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.fury.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.fury.token
  load_config_file       = false
}

module "fury" {
  source = "{{ .kubernetes.installerPath }}"

  cluster_name                          = var.cluster_name
  cluster_version                       = var.cluster_version
  cluster_log_retention_days            = var.cluster_log_retention_days
  cluster_endpoint_public_access        = var.cluster_endpoint_public_access
  cluster_endpoint_public_access_cidrs  = var.cluster_endpoint_public_access_cidrs
  cluster_endpoint_private_access       = var.cluster_endpoint_private_access
  cluster_endpoint_private_access_cidrs = var.cluster_endpoint_private_access_cidrs
  vpc_id                                = var.vpc_id
  subnets                               = var.subnets
  ssh_public_key                        = var.ssh_public_key
  node_pools                            = var.node_pools
  node_pools_launch_kind                = var.node_pools_launch_kind
  tags                                  = var.tags

  # Specific AWS variables.
  # Enables managing auth using these variables
  eks_map_users    = var.eks_map_users
  eks_map_roles    = var.eks_map_roles
  eks_map_accounts = var.eks_map_accounts
}
