
module "vpc-and-vpn" {
  source                   = "./modules/vpc-and-vpn"
  name                     = var.name
  network_cidr             = var.network_cidr
  public_subnetwork_cidrs  = var.public_subnetwork_cidrs
  private_subnetwork_cidrs = var.private_subnetwork_cidrs
  vpn_subnetwork_cidr      = var.vpn_subnetwork_cidr
  vpn_operator_cidrs       = var.vpn_operator_cidrs
  vpn_ssh_users            = var.vpn_ssh_users
}
