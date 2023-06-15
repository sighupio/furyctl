/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

terraform {
  backend "s3" {
    bucket = "{{ .terraform.backend.s3.bucketName }}"
    key    = "{{ .terraform.backend.s3.keyPrefix }}/infrastructure.json"
    region = "{{ .terraform.backend.s3.region }}"

    skip_region_validation = true
  }

  required_providers {
    aws = {
      source = "hashicorp/aws"
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

module "vpc" {
  source = "{{ .kubernetes.vpcInstallerPath }}"

  count = var.vpc_enabled ? 1 : 0

  name = var.name
  tags = var.tags

  cidr                     = var.cidr
  private_subnetwork_cidrs = var.vpc_private_subnetwork_cidrs
  public_subnetwork_cidrs  = var.vpc_public_subnetwork_cidrs

  # extra_ipv4_cidr_blocks = []
  # availability_zone_names = []
  # single_nat_gateway = false
  # one_nat_gateway_per_az = true
  # names_of_kubernetes_cluster_integrated_with_subnets = []
}

module "vpn" {
  source = "{{ .kubernetes.vpnInstallerPath }}"

  count = var.vpn_enabled ? 1 : 0

  name = var.name
  tags = var.tags

  vpc_id         = var.vpc_enabled ? one(module.vpc[*].vpc_id) : var.vpn_vpc_id
  public_subnets = var.vpc_enabled ? one(module.vpc[*].public_subnets) : var.vpn_public_subnets

  vpn_subnetwork_cidr    = var.vpn_subnetwork_cidr
  vpn_port               = var.vpn_port
  vpn_instances          = var.vpn_instances
  vpn_instance_type      = var.vpn_instance_type
  vpn_instance_disk_size = var.vpn_instance_disk_size
  vpn_operator_name      = var.vpn_operator_name
  vpn_dhparams_bits      = var.vpn_dhparams_bits
  vpn_operator_cidrs     = var.vpn_operator_cidrs
  vpn_ssh_users          = var.vpn_ssh_users

  # vpn_routes = []
}
