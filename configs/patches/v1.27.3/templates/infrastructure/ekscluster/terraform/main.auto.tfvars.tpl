/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

name = {{ .metadata.name | quote }}
{{- if and .spec.infrastructure (index .spec.infrastructure "vpc") }}
{{- $publicSubnetworkCIDRS := list }}
{{- $privateSubnetworkCIDRS := list }}
{{- range $p := .spec.infrastructure.vpc.network.subnetsCidrs.public }}
{{- $currCIDR := $p | quote}}
{{- $publicSubnetworkCIDRS = append $publicSubnetworkCIDRS $currCIDR }}
{{- end }}
{{- range $p := .spec.infrastructure.vpc.network.subnetsCidrs.private }}
{{- $currCIDR := $p | quote}}
{{- $privateSubnetworkCIDRS = append $privateSubnetworkCIDRS $currCIDR }}
{{- end }}
vpc_enabled = true
cidr = {{ .spec.infrastructure.vpc.network.cidr | quote }}
vpc_public_subnetwork_cidrs = [{{ $publicSubnetworkCIDRS | join ","}}]
vpc_private_subnetwork_cidrs = [{{ $privateSubnetworkCIDRS | join ","}}]
{{- else }}
vpc_enabled = false
{{- end }}
{{- if and .spec.infrastructure (index .spec.infrastructure "vpn") ((hasKeyAny .spec.infrastructure.vpn "instances") | ternary (and (index .spec.infrastructure.vpn "instances") (gt .spec.infrastructure.vpn.instances 0)) true) }}
vpn_enabled = true
vpn_subnetwork_cidr = {{ .spec.infrastructure.vpn.vpnClientsSubnetCidr | quote }}
{{- if index .spec.infrastructure.vpn "instances" }}
vpn_instances = {{ .spec.infrastructure.vpn.instances }}
{{- end }}
{{- if and (index .spec.infrastructure.vpn "port") (ne .spec.infrastructure.vpn.port 0) }}
vpn_port = {{ .spec.infrastructure.vpn.port }}
{{- end }}
{{- if and (index .spec.infrastructure.vpn "instanceType") (ne .spec.infrastructure.vpn.instanceType "") }}
vpn_instance_type = {{ .spec.infrastructure.vpn.instanceType | quote }}
{{- end }}
{{- if and (index .spec.infrastructure.vpn "diskSize") (ne .spec.infrastructure.vpn.diskSize 0) }}
vpn_instance_disk_size = {{ .spec.infrastructure.vpn.diskSize }}
{{- end }}
{{- if and (index .spec.infrastructure.vpn "operatorName") (ne .spec.infrastructure.vpn.operatorName "") }}
vpn_operator_name = {{ .spec.infrastructure.vpn.operatorName | quote }}
{{- end }}
{{- if and (index .spec.infrastructure.vpn "dhParamsBits") (ne .spec.infrastructure.vpn.dhParamsBits 0) }}
vpn_dhparams_bits = {{ .spec.infrastructure.vpn.dhParamsBits }}
{{- end }}
{{- if and (index .spec.infrastructure.vpn "bucketNamePrefix") (ne .spec.infrastructure.vpn.bucketNamePrefix "") }}
vpn_bucket_name_prefix = {{ .spec.infrastructure.vpn.bucketNamePrefix | quote }}
{{- end }}
{{- if gt (len .spec.infrastructure.vpn.ssh.allowedFromCidrs) 0 }}
{{- $allowedCIDRS := list }}
{{- range $p := .spec.infrastructure.vpn.ssh.allowedFromCidrs }}
{{- $currCIDR := $p | quote}}
{{- $allowedCIDRS = append $allowedCIDRS $currCIDR }}
{{- end }}
vpn_operator_cidrs = [{{ $allowedCIDRS | uniq | join ","}}]
{{- end }}
{{- if gt (len .spec.infrastructure.vpn.ssh.githubUsersName) 0 }}
{{- $sshUsers := list }}
{{- range $p := .spec.infrastructure.vpn.ssh.githubUsersName }}
{{- $currUser := $p | quote }}
{{- $sshUsers = append $sshUsers $currUser }}
{{- end }}
vpn_ssh_users = [{{ $sshUsers | join ","}}]
{{- end }}
{{- else }}
vpn_enabled = false
{{- end }}
