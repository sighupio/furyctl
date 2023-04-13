/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

provider "vsphere" {
  allow_unverified_ssl = true
  api_timeout          = 10
  vim_keep_alive       = 20
}

terraform {
  required_version = "0.15.4"
  required_providers {
    local   = "2.1.0"
    tls     = "3.1.0"
    vsphere = "1.26.0"
  }
}

resource "tls_private_key" "fury" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "local_file" "private" {
  sensitive_content    = tls_private_key.fury.private_key_pem
  filename             = "${path.module}/secrets/ssh-user"
  file_permission      = "0600"
  directory_permission = "0700"
}

resource "local_file" "public" {
  sensitive_content    = tls_private_key.fury.public_key_openssh
  filename             = "${path.module}/secrets/ssh-user.pub"
  file_permission      = "0644"
  directory_permission = "0700"
}

locals {
  tmp_ssh_public_keys = [for pub in var.ssh_public_keys : file(pub)]
  ssh_public_keys     = concat(local.tmp_ssh_public_keys, [tls_private_key.fury.public_key_openssh])
}

module "fury" {
  source = "https://github.com/sighupio/furyctl-provisioners/archive/refs/tags/v0.6.2.zip//furyctl-provisioners-0.6.2/modules/cluster/vsphere"

  name                        = var.name
  kube_version                = var.kube_version
  kube_control_plane_endpoint = var.kube_control_plane_endpoint
  etcd_version                = var.etcd_version
  oidc_client_id              = var.oidc_client_id
  oidc_issuer_url             = var.oidc_issuer_url
  oidc_ca_file                = var.oidc_ca_file
  cri_version                 = var.cri_version
  cri_proxy                   = var.cri_proxy
  cri_dns                     = var.cri_dns
  cri_mirrors                 = var.cri_mirrors

  env = var.env

  datacenter      = var.datacenter
  esxihosts       = var.esxihosts
  datastore       = var.datastore
  network         = var.network
  net_cidr        = var.net_cidr
  net_gateway     = var.net_gateway
  net_nameservers = var.net_nameservers
  net_domain      = var.net_domain
  ip_offset       = var.ip_offset

  enable_boundary_targets = var.enable_boundary_targets
  os_user                 = var.os_user
  ssh_public_keys         = local.ssh_public_keys

  kube_lb_count              = var.kube_lb_count
  kube_lb_template           = var.kube_lb_template
  kube_lb_custom_script_path = var.kube_lb_custom_script_path

  kube_master_count              = var.kube_master_count
  kube_master_cpu                = var.kube_master_cpu
  kube_master_mem                = var.kube_master_mem
  kube_master_disk_size          = var.kube_master_disk_size
  kube_master_template           = var.kube_master_template
  kube_master_labels             = var.kube_master_labels
  kube_master_taints             = var.kube_master_taints
  kube_master_custom_script_path = var.kube_master_custom_script_path

  kube_pod_cidr = var.kube_pod_cidr
  kube_svc_cidr = var.kube_svc_cidr

  kube_infra_count              = var.kube_infra_count
  kube_infra_cpu                = var.kube_infra_cpu
  kube_infra_mem                = var.kube_infra_mem
  kube_infra_disk_size          = var.kube_infra_disk_size
  kube_infra_template           = var.kube_infra_template
  kube_infra_labels             = var.kube_infra_labels
  kube_infra_taints             = var.kube_infra_taints
  kube_infra_custom_script_path = var.kube_infra_custom_script_path

  node_pools = var.node_pools
}
