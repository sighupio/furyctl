# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

module "vpc" {
  source = "github.com/sighupio/fury-eks-installer//modules/vpc?ref=v2.0.1"

  name = "${var.cluster_name}-vpc"

  cidr                     = "10.0.0.0/16"
  public_subnetwork_cidrs  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  private_subnetwork_cidrs = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]
}

module "vpn" {
  source = "github.com/sighupio/fury-eks-installer//modules/vpn?ref=v2.0.1"

  name = "${var.cluster_name}-vpn"

  vpc_id         = module.vpc.vpc_id
  public_subnets = module.vpc.public_subnets

  vpn_ssh_users       = ["SIGHUP-C-3PO"]
  vpn_subnetwork_cidr = "10.0.201.0/24"
}
