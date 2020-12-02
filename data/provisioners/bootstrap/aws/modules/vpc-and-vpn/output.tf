output "furyagent" {
  description = "furyagent.yml used by the vpn instance and ready to use to create a vpn profile"
  sensitive   = true
  value       = local.furyagent
}

output "vpn_ip" {
  description = "VPN instance IP"
  value       = aws_eip.vpn.public_ip
}
