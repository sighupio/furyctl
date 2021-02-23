/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

provider "aws" {
  version = "= 2.70.0"
}

provider "kubernetes" {
  version = "= 1.13.3"
}

provider "local" {
  version = "= 1.4.0"
}

provider "null" {
  version = "= 2.1.0"
}

provider "random" {
  version = "= 2.1.0"
}

provider "template" {
  version = "= 2.1.0"
}

module "fury" {
  source = "github.com/sighupio/fury-eks-installer//modules/eks?ref=v1.5.0"

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
