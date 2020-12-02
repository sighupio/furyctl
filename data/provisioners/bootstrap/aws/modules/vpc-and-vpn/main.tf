data "aws_region" "current" {}

data "aws_availability_zones" "available" {}

locals {
  cluster_name = "poc"
  # https://cloud-images.ubuntu.com/locator/ec2/
  # filter: 20.04 LTS eu- ebs-ssd 2020 amd64
  ubuntu_amis = {
    "eu-west-3" : "ami-098efdd0afb686fd5"
    "eu-west-2" : "ami-099ae17a6a688b1cc"
    "eu-west-1" : "ami-048309a44dad514df"
    "eu-south-1" : "ami-0e3c0649c89ccddc9"
    "eu-north-1" : "ami-01450210d4ebb3bab"
    "eu-central-1" : "ami-09f14afb2e15caab5"
  }
}

provider "local" {
  version = "~> 2.0"
}

provider "null" {
  version = "~> 3.0"
}

provider "aws" {
  version = "~> 3.19.0"
}
