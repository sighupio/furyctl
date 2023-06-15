terraform {
  backend "s3" {
    bucket = "{{ .spec.toolsConfiguration.terraform.state.s3.bucketName }}"
    key    = "{{ .spec.toolsConfiguration.terraform.state.s3.keyPrefix }}/distribution.json"
    region = "{{ .spec.toolsConfiguration.terraform.state.s3.region }}"

    skip_region_validation = true
  }
}

provider "aws" {
  region = "{{ .spec.toolsConfiguration.terraform.state.s3.region }}" # FIXME
}

data "aws_eks_cluster" "this" {
  name = "{{ .metadata.name }}"
}
