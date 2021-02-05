/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

output "furyagent" {
  description = "furyagent.yml used by the vpn instance and ready to use to create a vpn profile"
  sensitive   = true
  value       = module.vpc-and-vpn.furyagent
}

output "vpn_ip" {
  description = "VPN instance IP"
  value       = module.vpc-and-vpn.vpn_ip
}

output "vpn_operator_name" {
  description = "SSH Username to log into the VPN Instance"
  value       = var.vpn_operator_name
}

output "network_name" {
  description = "The name of the network"
  value       = module.vpc-and-vpn.network_name
}

output "public_subnets" {
  description = "List of names of public subnets"
  value       = module.vpc-and-vpn.public_subnets
}

output "public_subnets_cidr_blocks" {
  description = "List of cidr_blocks of public subnets"
  value       = module.vpc-and-vpn.public_subnets_cidr_blocks
}

output "private_subnets" {
  description = "List of names of private subnets"
  value       = module.vpc-and-vpn.private_subnets
}

output "private_subnets_cidr_blocks" {
  description = "List of cidr_blocks of private subnets"
  value       = module.vpc-and-vpn.private_subnets_cidr_blocks
}

output "cluster_subnet" {
  description = "Name of the cluster subnets"
  value       = module.vpc-and-vpn.cluster_subnet
}

output "cluster_subnet_cidr_blocks" {
  description = "List of cidr_blocks of private subnets"
  value       = module.vpc-and-vpn.cluster_subnet_cidr_blocks
}

output "additional_cluster_subnet" {
  description = "List of cidr_blocks of private subnets"
  value       = length(module.vpc-and-vpn.additional_cluster_subnet) == 1 ? module.vpc-and-vpn.additional_cluster_subnet[0] : []
}
