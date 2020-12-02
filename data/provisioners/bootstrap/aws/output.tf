output "furyagent" {
  description = "furyagent.yml used by the vpn instance and ready to use to create a vpn profile"
  sensitive   = true
  value       = module.vpc-and-vpn.furyagent
}

output "vpn_ip" {
  description = "VPN instance IP"
  value       = module.vpc-and-vpn.vpn_ip
}
