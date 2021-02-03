/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

module "vpc-and-vpn" {
  source = "https://github.com/sighupio/furyctl-provisioners/archive/v0.1.2.zip//furyctl-provisioners-0.1.2/modules/bootstrap/aws/vpc-and-vpn"

  name                     = var.name
  network_cidr             = var.network_cidr
  public_subnetwork_cidrs  = var.public_subnetwork_cidrs
  private_subnetwork_cidrs = var.private_subnetwork_cidrs
  vpn_subnetwork_cidr      = var.vpn_subnetwork_cidr
  vpn_port                 = var.vpn_port
  vpn_instance_type        = var.vpn_instance_type
  vpn_instance_disk_size   = var.vpn_instance_disk_size
  vpn_operator_name        = var.vpn_operator_name
  vpn_dhparams_bits        = var.vpn_dhparams_bits
  vpn_operator_cidrs       = var.vpn_operator_cidrs
  vpn_ssh_users            = var.vpn_ssh_users
}
