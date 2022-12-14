/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

terraform {
  experiments     = [module_variable_optional_attrs]

  backend "s3" {
    bucket = "{{ .spec.toolsConfiguration.terraform.state.s3.bucketName }}"
    key    = "{{ .spec.toolsConfiguration.terraform.state.s3.keyPrefix }}/cluster.json"
    region = "{{ .spec.toolsConfiguration.terraform.state.s3.region }}"
  }
}

module "fury" {
  source = "github.com/sighupio/fury-eks-installer//modules/eks?ref={{ .kubernetes.eks.installer }}"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version
  network         = var.network
  subnetworks     = var.subnetworks
  dmz_cidr_range  = var.dmz_cidr_range
  ssh_public_key  = var.ssh_public_key
  node_pools      = var.node_pools
  tags            = var.tags

  # Specific AWS variables.
  # Enables managing auth using these variables
  eks_map_users    = var.eks_map_users
  eks_map_roles    = var.eks_map_roles
  eks_map_accounts = var.eks_map_accounts
}
