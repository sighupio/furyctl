/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

variable "cluster_name" {
  type        = string
  description = "Unique cluster name. Used in multiple resources to identify your cluster resources"
}

variable "cluster_version" {
  type        = string
  description = "Kubernetes Cluster Version. Look at the cloud providers documentation to discover available versions. EKS example -> 1.16, GKE example -> 1.16.8-gke.9"
}

variable "network" {
  type        = string
  description = "Network where the Kubernetes cluster will be hosted"
}

variable "subnetworks" {
  type        = list
  description = "List of subnets where the cluster will be hosted"
}

variable "dmz_cidr_range" {
  type        = string
  description = "Network CIDR range from where cluster control plane will be accessible"
}

variable "ssh_public_key" {
  type        = string
  description = "Cluster administrator public ssh key. Used to access cluster nodes with the operator_ssh_user"
}

variable "node_pools" {
  description = "An object list defining node pools configurations"
  type = list(object({
    name          = string
    version       = string # null to use cluster_version
    min_size      = number
    max_size      = number
    instance_type = string
    max_pods      = number # null to use default upstream configuration
    volume_size   = number
    subnetworks   = list(string) # "" to use default upstream configuration
    labels        = map(string)
    taints        = list(string)
    tags          = map(string)
  }))
  default = []
}

variable "tags" {
  type        = map
  description = "The tags to apply to all resources"
  default     = {}
}

variable "resource_group_name" {
  type        = string
  description = "Resource group name where every resource will be placed. Required only in AKS installer (*)"
  default     = ""
}

// Google Cloud specific variables

variable "gke_network_project_id" {
  type        = string
  description = "[GKE] The project ID of the shared VPC's host (for shared vpc support)"
  default     = ""
}

variable "gke_master_ipv4_cidr_block" {
  type        = string
  description = "[GKE] The IP range in CIDR notation to use for the hosted master network"
  default     = "10.0.0.0/28"
}

variable "gke_add_additional_firewall_rules" {
  type        = bool
  description = "[GKE] Create additional firewall rules"
  default     = true
}

variable "gke_add_cluster_firewall_rules" {
  type        = bool
  description = "[GKE] Create additional firewall rules (Upstream GKE module)"
  default     = false
}

variable "gke_disable_default_snat" {
  type        = bool
  description = "[GKE] Whether to disable the default SNAT to support the private use of public IP addresses"
  default     = false
}
