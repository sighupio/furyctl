/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

terraform {
  required_version = "0.15.4"
  required_providers {
    local    = "2.0.0"
    null     = "3.0.0"
    aws      = "3.37.0"
    external = "2.0.0"
  }
}

module "vpc-and-vpn" {
  source = "github.com/sighupio/fury-eks-installer//modules/vpc-and-vpn?ref=v1.8.1"

  name                     = var.name
  network_cidr             = var.network_cidr
  public_subnetwork_cidrs  = var.public_subnetwork_cidrs
  private_subnetwork_cidrs = var.private_subnetwork_cidrs
  vpn_subnetwork_cidr      = var.vpn_subnetwork_cidr
  vpn_port                 = var.vpn_port
  vpn_instances            = var.vpn_instances
  vpn_instance_type        = var.vpn_instance_type
  vpn_instance_disk_size   = var.vpn_instance_disk_size
  vpn_operator_name        = var.vpn_operator_name
  vpn_dhparams_bits        = var.vpn_dhparams_bits
  vpn_operator_cidrs       = var.vpn_operator_cidrs
  vpn_ssh_users            = var.vpn_ssh_users
  tags                     = var.tags
}
