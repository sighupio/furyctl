/**
 * Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

output "furyagent" {
  description = "furyagent.yml used by the vpn instance and ready to use to create a vpn profile"
  sensitive   = true
  value       = module.vpc-and-vpn.furyagent
}

output "vpn_ip" {
  description = "VPN instance IPs"
  value       = module.vpc-and-vpn.vpn_ip
}

output "vpn_operator_name" {
  description = "SSH Username to log into the VPN Instance"
  value       = var.vpn_operator_name
}

output "vpc_id" {
  description = "The ID of the VPC"
  value       = module.vpc-and-vpn.vpc_id
}

output "vpc_cidr_block" {
  description = "The CIDR block of the VPC"
  value       = module.vpc-and-vpn.vpc_cidr_block
}

output "public_subnets" {
  description = "List of IDs of public subnets"
  value       = module.vpc-and-vpn.public_subnets
}

output "public_subnets_cidr_blocks" {
  description = "List of cidr_blocks of public subnets"
  value       = module.vpc-and-vpn.public_subnets_cidr_blocks
}

output "private_subnets" {
  description = "List of IDs of private subnets"
  value       = module.vpc-and-vpn.private_subnets
}

output "private_subnets_cidr_blocks" {
  description = "List of cidr_blocks of private subnets"
  value       = module.vpc-and-vpn.private_subnets_cidr_blocks
}
