/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

provider "local" {
  version = "2.0.0"
}

provider "null" {
  version = "3.0.0"
}

provider "external" {
  version = "2.0.0"
}

provider "random" {
  version = "3.0.1"
}

provider "google" {
  version = "3.55.0"
}

module "vpc-and-vpn" {
  source = "https://github.com/sighupio/furyctl-provisioners/archive/v0.3.0.zip//furyctl-provisioners-0.3.0/modules/bootstrap/gcp/vpc-and-vpn"

  name                             = var.name
  public_subnetwork_cidrs          = var.public_subnetwork_cidrs
  private_subnetwork_cidrs         = var.private_subnetwork_cidrs
  cluster_control_plane_cidr_block = var.cluster_control_plane_cidr_block
  cluster_subnetwork_cidr          = var.cluster_subnetwork_cidr
  cluster_pod_subnetwork_cidr      = var.cluster_pod_subnetwork_cidr
  cluster_service_subnetwork_cidr  = var.cluster_service_subnetwork_cidr
  vpn_subnetwork_cidr              = var.vpn_subnetwork_cidr
  tags                             = var.tags
  vpn_instances                    = var.vpn_instances
  vpn_port                         = var.vpn_port
  vpn_instance_type                = var.vpn_instance_type
  vpn_instance_disk_size           = var.vpn_instance_disk_size
  vpn_operator_name                = var.vpn_operator_name
  vpn_dhparams_bits                = var.vpn_dhparams_bits
  vpn_operator_cidrs               = var.vpn_operator_cidrs
  vpn_ssh_users                    = var.vpn_ssh_users
}
