output "ansible_inventory" {
  value       = module.fury.ansible_inventory
  description = "The ansible inventory to be used as hosts.ini file"
  sensitive   = true
}

output "haproxy_config" {
  value       = module.fury.haproxy_config
  description = "The ansible inventory to be used as hosts.ini file"
  sensitive   = true
}
