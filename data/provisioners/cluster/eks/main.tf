module "fury" {
  source = "github.com/sighupio/fury-eks-installer//modules/eks?ref=v1.2.0"

  cluster_name    = var.cluster_name
  cluster_version = var.cluster_version
  network         = var.network
  subnetworks     = var.subnetworks
  dmz_cidr_range  = var.dmz_cidr_range
  ssh_public_key  = var.ssh_public_key
  node_pools      = var.node_pools
  tags            = var.tags
}
