/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

variable "name" {
  description = "Name of the resources. Used as cluster name"
  type        = string
}

variable "network_cidr" {
  description = "VPC Network CIDR"
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

variable "vpn_subnetwork_cidr" {
  description = "VPN Subnet CIDR, should be different from the network_cidr"
  type        = string
}

variable "vpn_port" {
  description = "VPN Server Port"
  type        = number
  default     = 1194
}

variable "vpn_instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t3.micro"
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
