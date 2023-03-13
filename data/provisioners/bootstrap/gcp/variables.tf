/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

variable "provider_project" {
  description = "Name of the Provider's project"
  type        = string
}

variable "provider_region" {
  description = "Name of the Provider's region"
  type        = string
}

variable "name" {
  description = "Name of the resources. Used as cluster name"
  type        = string
}

variable "public_subnetwork_cidrs" {
  description = "Public subnet CIDRs"
  type        = list(string)
}

variable "private_subnetwork_cidrs" {
  description = "Private subnet CIDRs"
  type        = list(string)
}

variable "cluster_control_plane_cidr_block" {
  description = "Private subnet CIDR hosting the GKE control plane"
  type        = string
  default     = "10.0.0.0/28"
}

variable "cluster_subnetwork_cidr" {
  description = "Private subnet CIDR"
  type        = string
}

variable "cluster_pod_subnetwork_cidr" {
  description = "Private subnet CIDR"
  type        = string
}

variable "cluster_service_subnetwork_cidr" {
  description = "Private subnet CIDR"
  type        = string
}

variable "tags" {
  description = "A map of tags to add to all resources"
  type        = map(string)
  default     = {}
}

variable "vpn_subnetwork_cidr" {
  description = "VPN Subnet CIDR, should be different from the network_cidr"
  type        = string
}

variable "vpn_instances" {
  description = "VPN Servers"
  type        = number
  default     = 1
}

variable "vpn_port" {
  description = "VPN Server Port"
  type        = number
  default     = 1194
}

variable "vpn_instance_type" {
  description = "GCP instance type"
  type        = string
  default     = "n1-standard-1"
}

variable "vpn_instance_disk_size" {
  description = "VPN main disk size"
  type        = number
  default     = 50
}

variable "vpn_operator_name" {
  description = "VPN operator name. Used to log into the instance via SSH"
  type        = string
  default     = "sighup"
}

variable "vpn_dhparams_bits" {
  description = "Diffie–Hellman (D-H) key size in bytes"
  type        = number
  default     = 2048
}

variable "vpn_operator_cidrs" {
  description = "VPN Operator cidrs. Used to log into the instance via SSH"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "vpn_ssh_users" {
  description = "GitHub users id to sync public rsa keys. Example angelbarrera92"
  type        = list(string)
}
