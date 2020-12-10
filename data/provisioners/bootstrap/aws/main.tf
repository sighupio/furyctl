
module "vpc-and-vpn" {
  source                   = "https://github.com/sighupio/furyctl-provisioners/archive/v0.1.0.zip//furyctl-provisioners-0.1.0/modules/bootstrap/aws/vpc-and-vpn"
  name                     = var.name
  network_cidr             = var.network_cidr
  public_subnetwork_cidrs  = var.public_subnetwork_cidrs
  private_subnetwork_cidrs = var.private_subnetwork_cidrs
  vpn_subnetwork_cidr      = var.vpn_subnetwork_cidr
  vpn_operator_cidrs       = var.vpn_operator_cidrs
  vpn_ssh_users            = var.vpn_ssh_users
}
