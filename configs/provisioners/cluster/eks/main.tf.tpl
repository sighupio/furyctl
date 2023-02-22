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
  }
}

module "fury" {
  source = "{{ .kubernetes.installerPath }}"

  cluster_name               = var.cluster_name
  cluster_version            = var.cluster_version
  cluster_log_retention_days = var.cluster_log_retention_days
  network                    = var.network
  subnetworks                = var.subnetworks
  dmz_cidr_range             = var.dmz_cidr_range
  ssh_public_key             = var.ssh_public_key
  node_pools                 = var.node_pools
  node_pools_launch_kind     = var.node_pools_launch_kind
  tags                       = var.tags

  # Specific AWS variables.
  # Enables managing auth using these variables
  eks_map_users    = var.eks_map_users
  eks_map_roles    = var.eks_map_roles
  eks_map_accounts = var.eks_map_accounts
}
