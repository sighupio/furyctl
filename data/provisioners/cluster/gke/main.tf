/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

module "fury" {
  source = "github.com/sighupio/fury-gke-installer//modules/gke?ref=v1.4.1"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version
  network         = var.network
  subnetworks     = var.subnetworks
  dmz_cidr_range  = var.dmz_cidr_range
  ssh_public_key  = var.ssh_public_key
  node_pools      = var.node_pools
  tags            = var.tags

  # Specific GKE variables.
  gke_master_ipv4_cidr_block        = var.gke_master_ipv4_cidr_block
  gke_add_additional_firewall_rules = var.gke_add_additional_firewall_rules
  gke_add_cluster_firewall_rules    = var.gke_add_cluster_firewall_rules
  gke_disable_default_snat          = var.gke_disable_default_snat
}
