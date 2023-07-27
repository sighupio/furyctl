> WARNING: The following guide is only to move infrastructure and/or kubernetes phase from v0.11.1 to v0.25.1 furyctl version

> WARNING: only `s3` terraform backend is supported

# Migration steps
1. Update EKS cluster to v1.25 using the latest furyctl legacy version v0.11.1
2. Using furyctl 0.25.1 execute`furyctl create config -v 1.25.5 -k EKSCluster`
3. Copy configuration values from `bootstrap.yml` into `furyctl.yml` using the following mapping:


| `boostrap.yml`             | `furyctl.yml`                                          | Note                                                                                                          |
|----------------------------|--------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| `metadata.name`            | `metadata.name`                                        ||
| `spec.networkCIDR`         | `spec.infrastructure.vpc.network.cidr`                 ||
| `spec.publicSubnetsCIDRs`  | `spec.infrastructure.vpc.network.subnetsCidrs.public`  ||
| `spec.privateSubnetsCIDRs` | `spec.infrastructure.vpc.network.subnetsCidrs.private` ||
| `spec.vpn.instance`        | `spec.infrastructure.vpn.instances`                    ||
| `spec.vpn.port`            | `spec.infrastructure.vpn.port`                         ||
| `spec.vpn.instanceType`    | `spec.infrastructure.vpn.instanceType`                 ||
| `spec.vpn.diskSize`        | `spec.infrastructure.vpn.diskSize`                     ||
| `spec.vpn.operatorName`    | `spec.infrastructure.vpn.operatorName`                 ||
| `spec.vpn.dhParamsBits`    | `spec.infrastructure.vpn.dhParamsBits`                 ||
| `spec.vpn.subnetCIDR`      | `spec.infrastructure.vpn.vpnClientsSubnetCidr`         ||
| `spec.vpn.sshUsers`        | `spec.infrastructure.vpn.ssh.githubUsersName`          ||
| `spec.vpn.operatorCIDRs`   | `spec.infrastructure.vpn.ssh.allowedFromCidrs`         ||
| `executor.state.backend.bucket`  | `spec.toolsConfiguration.terraform.state.s3.bucketName`||
| `executor.state.backend.key`  | `spec.toolsConfiguration.terraform.state.s3.keyPrefix`| On S3 move/rename the ../../../bin/terraform/1.4.6/terraform state file to `infrastructure.json` under the folder specified by `keyPrefix` |
| `executor.state.backend.region`  | `spec.toolsConfiguration.terraform.state.s3.region`||

4. Set `spec.infrastructure.vpn.bucketNamePrefix: METADATA_NAME-bootstrap-bucket-` where METADATA_NAME is the value of `metadata.name` in `furyctl.yml`
5. Copy configuration values from `cluster.yml` into `furyctl.yml` using the following mapping:

| `cluster.yml`                                            | `furyctl.yml`                                                                                                                                        | Note                                                                                                   |
|----------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|
| `spec.network`                                           | `spec.kubernetes.vpcId`                                                                                                                              ||
| `spec.subnetworks`                                       | `spec.kubernetes.subnetIds`                                                                                                                          ||
| `spec.dmzCIDRRange`                                      | `spec.kubernetes.apiServer.privateAccessCidrs`                                                                                                       ||
| `spec.sshPublicKey`                                      | `spec.kubernetes.nodeAllowedSshPublicKey`                                                                                                            ||
| `spec.nodePools[*].name`                                 | `spec.kubernetes.nodePool[*].name`                                                                                                                   | keep the same order when report the node pools to avoid replacement                                    |
| `spec.nodePools[*].os`                                   | `spec.kubernetes.nodePool[*].ami.id`                                                                                                                 ||
| `spec.nodePools[*].targetGroups`                         | `spec.kubernetes.nodePool[*].attachedTargetGroups`                                                                                                   ||
| `spec.nodePools[*].containerRuntime`                     | `spec.kubernetes.nodePool[*].containerRuntime`                                                                                                       ||
| `spec.nodePools[*].minSize`                              | `spec.kubernetes.nodePool[*].size.min`                                                                                                               ||
| `spec.nodePools[*].maxSize`                              | `spec.kubernetes.nodePool[*].size.max`                                                                                                               ||
| `spec.nodePools[*].instanceType`                         | `spec.kubernetes.nodePool[*].instance.type`                                                                                                          ||
| `spec.nodePools[*].spotInstance`                         | `spec.kubernetes.nodePool[*].instance.spot`                                                                                                          ||
| `spec.nodePools[*].volumeSize`                           | `spec.kubernetes.nodePool[*].instance.volumeSize`                                                                                                    ||
| `spec.nodePools[*].maxPods`                              | `null`                                                                                                                                               ||
| `spec.nodePools[*].nodePoolsLaunchKind`                  | `spec.kubernetes.nodePool[*].nodePoolsLaunchKind`                                                                                                    |                                                                                                        |
| `spec.nodePools[*].labels`                               | `spec.kubernetes.nodePool[*].labels`                                                                                                                 ||
| `spec.nodePools[*].taints`                               | `spec.kubernetes.nodePool[*].taints`                                                                                                                 ||
| `spec.nodePools[*].tags`                                 | `spec.kubernetes.nodePool[*].tags`                                                                                                                   ||
| `spec.nodePools[*].additionalFirewallRules[*].name`      | `spec.kubernetes.nodePool[*].additionalFirewallRules.cidrBlocks[*].name`                                                                             ||
| `spec.nodePools[*].additionalFirewallRules[*].direction` | `spec.kubernetes.nodePool[*].additionalFirewallRules.cidrBlocks[*].type`                                                                             ||
| `spec.nodePools[*].additionalFirewallRules[*].cidrBlock` | `spec.kubernetes.nodePool[*].additionalFirewallRules.cidrBlocks[*].cidrBlocks`                                                                       | From string to list of string                                                                          |
| `spec.nodePools[*].additionalFirewallRules[*].protocol`  | `spec.kubernetes.nodePool[*].additionalFirewallRules.cidrBlocks[*].protocol`                                                                         |                                                                                                        |
| `spec.nodePools[*].additionalFirewallRules[*].ports`     | `spec.kubernetes.nodePool[*].additionalFirewallRules.cidrBlocks[*].ports.from`<br/>`spec.kubernetes.nodePool[*].additionalFirewallRules[*].ports.to` |                                                                                                        |
| `spec.nodePools[*].additionalFirewallRules[*].tags`      | `spec.kubernetes.nodePool[*].additionalFirewallRules.cidrBlocks[*].tags`                                                                             | Make sure the tag `node.kubernetes.io/role` is present                                                 |
| `spec.tags`                                              | `null`                                                                                                                                               ||
| `spec.auth`                                              | `spec.kubernetes.awsAuth`                                                                                                                            ||
| `spec.logRetentionDays`                                  | `spec.kubernetes.logRetentionDays`                                                                                                                   ||
| `executor.state.backend.bucket`                          | `spec.toolsConfiguration.terraform.state.s3.bucketName`                                                                                              ||
| `executor.state.backend.key`                             | `spec.toolsConfiguration.terraform.state.s3.keyPrefix`                                                                                               | On S3 move/rename the ../../../bin/terraform/1.4.6/terraform state file to `cluster.json` under the folder specified by `keyPrefix` |
| `executor.state.backend.region`                          | `spec.toolsConfiguration.terraform.state.s3.region`                                                                                                  ||

> NOTE: in case you have only the `cluster.yml`, set `spec.kubernetes.subnetsIds` on `furyctl.yml`

The following `distribution` fields are the minimum required to be present on `furyctl.yml`. 

Adjust value according your environment

```yaml
spec:
  distribution:
    common:
      nodeSelector:
        node.kubernetes.io/role: infra
      tolerations:
        - effect: NoSchedule
          key: node.kubernetes.io/role
          value: infra
    modules:
      ingress:
        baseDomain: internal.example.dev
        nginx:
          type: dual
        dns:
          public:
            name: "example.dev"
            create: false
          private:
            name: "internal.example.dev"
            create: false
      logging:
        type: opensearch
        opensearch:
          type: single
      policy:
        type: gatekeeper
      dr:
        type: none
        velero:
          eks:
            bucketName: example-velero
            region: eu-west-1
```

6. Using the furyctl v0.25.1 
```shell
#Ensure your aws credential has been set
furyctl create cluster --dry-run --phase infrastructure -o $(pwd)

# where METADATA_NAME is the value of `metadata.name` in `furyctl.yml`
cd .furyctl/METADATA_NAME/infrastructure/terraform

export VPC_AND_VPN_MODULE_NAME=vpc-and-vpn
export VPC_MODULE_NAME='vpc[0]'
export VPN_MODULE_NAME='vpn[0]'
#export VPN_INSTANCES=2
#export LENGTH_PRIVATE_SUBNETS=3
#export LENGTH_PUBLIC_SUBNETS=3

echo "../../../bin/terraform/1.4.6/../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_vpc.this[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_vpc.this[0]'" | sh

for COUNT in {0..$((${VPN_INSTANCES:-2}-1))}; do
echo "../../../bin/terraform/1.4.6/../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_eip_association.vpn[$COUNT]' 'module.${VPN_MODULE_NAME}.aws_eip_association.vpn[$COUNT]'" | sh
echo "../../../bin/terraform/1.4.6/../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_instance.vpn[$COUNT]' 'module.${VPN_MODULE_NAME}.aws_instance.vpn[$COUNT]'" | sh
echo "../../../bin/terraform/1.4.6/../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_eip.vpn[$COUNT]' 'module.${VPN_MODULE_NAME}.aws_eip.vpn[$COUNT]'" | sh
done

echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_route_table.private[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_route_table.private[0]'" | sh
for COUNT in {0..$((${LENGTH_PRIVATE_SUBNETS:-3}-1))}; do
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_route_table_association.private[$COUNT]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_route_table_association.private[$COUNT]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_subnet.private[$COUNT]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_subnet.private[$COUNT]'" | sh
done

echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_route_table.public[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_route_table.public[0]'" | sh
for COUNT in {0..$((${LENGTH_PUBLIC_SUBNETS:-3}-1))}; do
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_subnet.public[$COUNT]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_subnet.public[$COUNT]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_route_table_association.public[$COUNT]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_route_table_association.public[$COUNT]'" | sh
done

echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_eip.nat[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_eip.nat[0]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_internet_gateway.this[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_internet_gateway.this[0]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_nat_gateway.this[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_nat_gateway.this[0]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_route.private_nat_gateway[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_route.private_nat_gateway[0]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.module.vpc.aws_route.public_internet_gateway[0]' 'module.${VPC_MODULE_NAME}.module.vpc.aws_route.public_internet_gateway[0]'" | sh


echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_iam_access_key.furyagent' 'module.${VPN_MODULE_NAME}.aws_iam_access_key.furyagent'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_iam_policy.furyagent' 'module.${VPN_MODULE_NAME}.aws_iam_policy.furyagent'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_iam_policy_attachment.furyagent' 'module.${VPN_MODULE_NAME}.aws_iam_policy_attachment.furyagent'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_iam_user.furyagent' 'module.${VPN_MODULE_NAME}.aws_iam_user.furyagent'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_s3_bucket.furyagent' 'module.${VPN_MODULE_NAME}.aws_s3_bucket.furyagent'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_security_group.vpn' 'module.${VPN_MODULE_NAME}.aws_security_group.vpn'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_security_group_rule.vpn' 'module.${VPN_MODULE_NAME}.aws_security_group_rule.vpn'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_security_group_rule.vpn_egress' 'module.${VPN_MODULE_NAME}.aws_security_group_rule.vpn_egress'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.aws_security_group_rule.vpn_ssh' 'module.${VPN_MODULE_NAME}.aws_security_group_rule.vpn_ssh'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.local_file.furyagent' 'module.${VPN_MODULE_NAME}.local_file.furyagent'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.local_file.sshkeys' 'module.${VPN_MODULE_NAME}.local_file.sshkeys'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.null_resource.init' 'module.${VPN_MODULE_NAME}.null_resource.init'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${VPC_AND_VPN_MODULE_NAME}.null_resource.ssh_users' 'module.${VPN_MODULE_NAME}.null_resource.ssh_users'" | sh

cd ../../../..

furyctl create cluster --phase infrastructure -o $(pwd)
furyctl create cluster --dry-run --phase kubernetes -o $(pwd)

# where METADATA_NAME is the value of `metadata.name` in `furyctl.yml`
cd .furyctl/METADATA_NAME/kubernetes/terraform

export EKS_MODULE_NAME=fury
echo "../../../bin/terraform/1.4.6/terraform state rm 'module.${EKS_MODULE_NAME}.module.cluster.kubernetes_config_map.aws_auth[0]'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${EKS_MODULE_NAME}.aws_security_group.nodes' 'module.${EKS_MODULE_NAME}.aws_security_group.node_pool_shared'" | sh
echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${EKS_MODULE_NAME}.aws_security_group_rule.ssh_from_dmz_to_nodes' 'module.${EKS_MODULE_NAME}.aws_security_group_rule.ssh_to_nodes'" | sh

# Make sure to run on bash because zsh is not supported
#!/bin/bash
nodepools=(`cat ../../../../furyctl.yaml | ../../../bin/yq/4.34.1/yq -e '.spec.kubernetes.nodePools.[].name'`)
for i in ${!nodepools[@]};do
 echo "../../../bin/terraform/1.4.6/terraform state mv 'module.${EKS_MODULE_NAME}.aws_security_group.node_pool[$i]' 'module.${EKS_MODULE_NAME}.aws_security_group.node_pool[\"${nodepools[$i]}\"]'" | sh
done

echo "../../../bin/terraform/1.4.6/terraform import "module.${EKS_MODULE_NAME}.module.cluster.kubernetes_config_map.aws_auth[0]" 'kube-system/aws-auth'" | sh
cd ../../../..
furyctl create cluster --phase kubernetes  -o $(pwd) 
```