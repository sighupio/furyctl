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

variable "vpn_ssh_users" {
  description = "GitHub users id to sync public rsa keys. Example angelbarrera92"
  type        = list(string)
}

variable "vpn_operator_cidrs" {
  description = "VPN Operator cidrs. Used to log into the instance via SSH"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}