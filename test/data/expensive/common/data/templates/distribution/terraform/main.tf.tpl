terraform {
  backend "s3" {
    bucket = "{{ .spec.toolsConfiguration.terraform.state.s3.bucketName }}"
    key    = "{{ .spec.toolsConfiguration.terraform.state.s3.keyPrefix }}/distribution.json"
    region = "{{ .spec.toolsConfiguration.terraform.state.s3.region }}"

    {{- if index .terraform.backend.s3 "skipRegionValidation" }}
      skip_region_validation = {{ default false .terraform.backend.s3.skipRegionValidation }}
    {{- end }}
  }
}

provider "aws" {
  region = "{{ .spec.toolsConfiguration.terraform.state.s3.region }}" # FIXME
}

data "aws_eks_cluster" "this" {
  name = "{{ .metadata.name }}"
}
