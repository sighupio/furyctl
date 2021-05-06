/**
 * Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

variable "name" {
  type        = string
  description = "Cluster name"
}

variable "kube_version" {
  type        = string
  description = "Kubernetes version"
}

variable "etcd_version" {
  type        = string
  description = "ETCD Version to install"
  default     = "v3.4.15"
}

variable "oidc_client_id" {
  type        = string
  description = "OIDC Client ID"
  default     = ""
}

variable "oidc_issuer_url" {
  type        = string
  description = "OIDC Issuer URL"
  default     = ""
}

variable "oidc_ca_file" {
  type        = string
  description = "OIDC CA File"
  default     = ""
}

variable "cri_proxy" {
  type        = string
  description = "CRI Proxy configuration"
  default     = ""
}

variable "cri_version" {
  type        = string
  description = "CRI Version"
  default     = ""
}

variable "cri_dns" {
  type        = list(string)
  description = "DNS Servers for the CRI"
  default     = []
}

variable "cri_mirrors" {
  type        = list(string)
  description = "Mirror Servers for the CRI"
  default     = []
}

variable "env" {
  type        = string
  description = "Cluster environment"
}

variable "datacenter" {
  type        = string
  description = "Datacenter Name as seen in vCenter"
}

variable "esxihosts" {
  type        = list(string)
  description = "Hostname where to create the VMs"
}

variable "datastore" {
  type        = string
  description = "Datastore where to create the VMs"
}

variable "network" {
  type        = string
  description = "vNetwork where to create the VMs"
}

variable "net_cidr" {
  type        = string
  description = "Base IP CIDR address used to calculate the VMs IPs."
}

variable "net_gateway" {
  type        = string
  default     = ""
  description = "The IP address of the default gateway to set on the VMs. If not specified the value used will be the first IP of the subnet."
}

variable "net_nameservers" {
  type        = list(string)
  default     = []
  description = "list of nameservers to configure on the VMs. If not set the gateway will be used as DNS."
}

variable "net_domain" {
  type        = string
  default     = "localdomain"
  description = "DNS search domain names to configure on the VMs."
}

variable "ip_offset" {
  type        = number
  default     = 0
  description = "Number to sum at every IP calculation. Enable deploying multiple clusters in the same network"
}

variable "enable_boundary_targets" {
  description = "Enable boundary on all the nodes"
  type        = bool
  default     = false
}

variable "os_user" {
  type        = string
  default     = "sighup"
  description = "Operating System User to use"
}

variable "ssh_public_keys" {
  type        = list(string)
  description = "List of public SSH keys authorized to connect to the VMs"
}

variable "kube_lb_count" {
  type        = number
  description = "Number of HAproxy Load Balancers VMs to create"
}

variable "kube_lb_template" {
  type        = string
  description = "VM template name to clone from to create the Loadbalancers VMs"
}

variable "kube_lb_custom_script_path" {
  type        = string
  description = "Local path of the script that must execute on first boot on lb nodes"
  default     = ""
}

variable "kube_master_count" {
  type        = number
  default     = 1
  description = "Number of Kubernetes master nodes"
}

variable "kube_master_cpu" {
  type        = number
  default     = 2
  description = "Kubernetes master nodes vCPU count"
}

variable "kube_master_mem" {
  type        = number
  default     = 4096
  description = "Kubernetes master nodes memory count in MB"
}

variable "kube_master_disk_size" {
  type        = number
  default     = 80
  description = "Kubernetes master nodes disk size in GB"
}

variable "kube_master_template" {
  type        = string
  description = "VM template name to clone from for Kubernetes master nodes"
}

variable "kube_master_labels" {
  type        = map(string)
  description = "Kubernetes labels to set at the master nodes"
  default     = {}
}

variable "kube_master_taints" {
  type        = list(string)
  description = "Kubernetes taints to set at the master nodes"
  default     = []
}

variable "kube_master_custom_script_path" {
  type        = string
  description = "Local path of the script that must execute on first boot on master nodes"
  default     = ""
}

variable "kube_pod_cidr" {
  type        = string
  description = "POD CIDR to set on kubelet configuration"
  default     = "172.21.0.0/16"
}

variable "kube_svc_cidr" {
  type        = string
  description = "Services CIDR to set on kubelet configuration"
  default     = "172.23.0.0/16"
}

variable "kube_infra_count" {
  type        = number
  default     = 1
  description = "Number of Kubernetes infra nodes. It can be zero."
}

variable "kube_infra_cpu" {
  type        = string
  default     = 2
  description = "Kubernetes infra nodes vCPU count"
}

variable "kube_infra_mem" {
  type        = string
  default     = 8192
  description = "Kubernetes infra nodes memory count in MB"
}

variable "kube_infra_disk_size" {
  type        = string
  default     = 100
  description = "Kubernetes infra nodes disk size in GB"
}

variable "kube_infra_template" {
  type        = string
  description = "VM template name to clone from for Kubernetes infra nodes"
}

variable "kube_infra_labels" {
  type        = map(string)
  description = "Kubernetes labels to set at the infra nodes"
  default     = {}
}

variable "kube_infra_taints" {
  type        = list(string)
  description = "Kubernetes taints to set at the infra nodes"
  default     = []
}

variable "kube_infra_custom_script_path" {
  type        = string
  description = "Local path of the script that must execute on first boot on infra nodes"
  default     = ""
}

variable "node_pools" {
  description = "Node-pool definitions"
  type = list(object({
    role               = string
    template           = string
    count              = number
    memory             = number
    cpu                = number
    disk_size          = number
    labels             = map(string)
    taints             = list(string)
    custom_script_path = string
  }))
  default = []
}
