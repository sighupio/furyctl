/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

terraform {
  required_version = "0.15.4"
  required_providers {
    local    = "= 2.0.0"
    null     = "= 3.0.0"
    external = "= 2.0.0"
    random   = "3.0.1"
    google   = "3.55.0"
  }
}

provider "google" {
  project = var.provider_project
  region  = var.provider_region
}

module "vpc-and-vpn" {
  source = "github.com/sighupio/fury-gke-installer//modules/vpc-and-vpn?ref=v1.10.0"

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
