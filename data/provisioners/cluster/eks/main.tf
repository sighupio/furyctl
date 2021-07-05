/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

terraform {
  required_version = "0.15.4"
  required_providers {
    aws        = "= 3.37.0"
    kubernetes = "= 1.13.3"
    local      = "= 1.4.0"
    null       = "= 2.1.0"
    random     = "= 2.1.0"
    template   = "= 2.1.0"
  }
}

module "fury" {
  source = "github.com/sighupio/fury-eks-installer//modules/eks?ref=v1.7.1"

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
