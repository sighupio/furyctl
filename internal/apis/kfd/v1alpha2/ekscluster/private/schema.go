// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package private contains furyctl's curated, hand-maintained view of the
// EKSCluster furyctl.yaml (apiVersion kfd.sighup.io/v1alpha2).
//
// It is called "private" for historical reasons: besides the fields furyctl
// reads from the user's furyctl.yaml, it also models the EKS-internal fields
// that furyctl itself fills in by injecting infrastructure (Terraform/OpenTofu)
// outputs before rendering templates — see common/distribution.go
// (injectDataPreTf / InjectDataPostTf). Those values (IAM role ARNs, VPC id)
// are never user input.
//
// Only the fields furyctl actually uses are modeled. The full furyctl.yaml is
// validated at runtime against the JSON schema shipped by the distribution
// (see internal/config.Validate). Keep this readable: it should read like a
// furyctl.yaml skeleton.
package private

// String-like domain types. Some are referenced by name in furyctl (used as
// conversions, e.g. private.TypesAwsArn(...)); the others keep the string(...)
// conversions at the call sites meaningful while behaving as plain strings.
type (
	TypesAwsArn          string
	TypesAwsSubnetId     string
	TypesAwsVpcId        string
	TypesCidr            string
	TypesAwsRegion       string
	TypesAwsS3BucketName string
	TypesAwsS3KeyPrefix  string
	Kind                 string // furyctl.yaml kind discriminator (EKSCluster)
)

// EksclusterKfdV1Alpha2 is furyctl's read+inject view of an EKSCluster config.
type EksclusterKfdV1Alpha2 struct {
	Kind     Kind     `yaml:"kind"`
	Metadata Metadata `yaml:"metadata"`
	Spec     Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Spec struct {
	Region             TypesAwsRegion         `yaml:"region"`
	Infrastructure     *SpecInfrastructure    `yaml:"infrastructure,omitempty"`
	Kubernetes         SpecKubernetes         `yaml:"kubernetes"`
	Distribution       SpecDistribution       `yaml:"distribution"`
	ToolsConfiguration SpecToolsConfiguration `yaml:"toolsConfiguration"`
}

// --- infrastructure ---

type SpecInfrastructure struct {
	Vpc *SpecInfrastructureVpc `yaml:"vpc,omitempty"`
	Vpn *SpecInfrastructureVpn `yaml:"vpn,omitempty"`
}

type SpecInfrastructureVpc struct {
	Network SpecInfrastructureVpcNetwork `yaml:"network"`
}

type SpecInfrastructureVpcNetwork struct {
	Cidr TypesCidr `yaml:"cidr"`
}

// TypesTcpPort is a TCP port number.
type TypesTcpPort int

type SpecInfrastructureVpn struct {
	BucketNamePrefix    *string       `yaml:"bucketNamePrefix,omitempty"`
	IamUserNameOverride *string       `yaml:"iamUserNameOverride,omitempty"`
	Instances           *int          `yaml:"instances,omitempty"`
	OperatorName        *string       `yaml:"operatorName,omitempty"`
	Port                *TypesTcpPort `yaml:"port,omitempty"`
}

// --- kubernetes ---

type SpecKubernetes struct {
	ApiServer                        SpecKubernetesAPIServer  `yaml:"apiServer"`
	ClusterIAMRoleNamePrefixOverride *string                  `yaml:"clusterIAMRoleNamePrefixOverride,omitempty"`
	NodePools                        []SpecKubernetesNodePool `yaml:"nodePools"`
	SubnetIds                        []TypesAwsSubnetId       `yaml:"subnetIds,omitempty"`
	VpcId                            *TypesAwsVpcId           `yaml:"vpcId,omitempty"`
	WorkersIAMRoleNamePrefixOverride *string                  `yaml:"workersIAMRoleNamePrefixOverride,omitempty"`
}

type SpecKubernetesAPIServer struct {
	PrivateAccess bool `yaml:"privateAccess"`
	PublicAccess  bool `yaml:"publicAccess"`
}

type SpecKubernetesNodePool struct {
	Size SpecKubernetesNodePoolSize `yaml:"size"`
}

type SpecKubernetesNodePoolSize struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// --- distribution: read (Dr.Type, Aws overrides) + injection targets ---

type SpecDistribution struct {
	Modules SpecDistributionModules `yaml:"modules"`
}

type SpecDistributionModules struct {
	Aws     *SpecDistributionModulesAws    `yaml:"aws,omitempty"`
	Dr      SpecDistributionModulesDr      `yaml:"dr"`
	Ingress SpecDistributionModulesIngress `yaml:"ingress"`
}

type SpecDistributionModulesAws struct {
	ClusterAutoscaler      SpecDistributionModulesAwsClusterAutoscaler      `yaml:"clusterAutoscaler"`
	EbsCsiDriver           SpecDistributionModulesAwsEbsCsiDriver           `yaml:"ebsCsiDriver"`
	LoadBalancerController SpecDistributionModulesAwsLoadBalancerController `yaml:"loadBalancerController"`
}

type SpecDistributionModulesAwsEbsCsiDriver struct {
	IamRoleArn TypesAwsArn                        `yaml:"iamRoleArn"`
	Overrides  *ComponentOverridesWithIAMRoleName `yaml:"overrides,omitempty"`
}

type SpecDistributionModulesAwsClusterAutoscaler struct {
	IamRoleArn TypesAwsArn                        `yaml:"iamRoleArn"`
	Overrides  *ComponentOverridesWithIAMRoleName `yaml:"overrides,omitempty"`
}

type SpecDistributionModulesAwsLoadBalancerController struct {
	IamRoleArn TypesAwsArn                        `yaml:"iamRoleArn"`
	Overrides  *ComponentOverridesWithIAMRoleName `yaml:"overrides,omitempty"`
}

type ComponentOverridesWithIAMRoleName struct {
	IamRoleName *string `yaml:"iamRoleName,omitempty"`
}

type SpecDistributionModulesDr struct {
	Type   string                           `yaml:"type"`
	Velero *SpecDistributionModulesDrVelero `yaml:"velero,omitempty"`
}

type SpecDistributionModulesDrVelero struct {
	Eks SpecDistributionModulesDrVeleroEks `yaml:"eks"`
}

type SpecDistributionModulesDrVeleroEks struct {
	IamRoleArn TypesAwsArn `yaml:"iamRoleArn"`
}

type SpecDistributionModulesIngress struct {
	CertManager SpecDistributionModulesIngressCertManager `yaml:"certManager"`
	Dns         *SpecDistributionModulesIngressDNS        `yaml:"dns,omitempty"`
	ExternalDns SpecDistributionModulesIngressExternalDNS `yaml:"externalDns"`
}

type SpecDistributionModulesIngressCertManager struct {
	ClusterIssuer SpecDistributionModulesIngressCertManagerClusterIssuer `yaml:"clusterIssuer"`
}

type SpecDistributionModulesIngressCertManagerClusterIssuer struct {
	Route53 SpecDistributionModulesIngressClusterIssuerRoute53 `yaml:"route53"`
}

type SpecDistributionModulesIngressClusterIssuerRoute53 struct {
	IamRoleArn   TypesAwsArn `yaml:"iamRoleArn"`
	HostedZoneId string      `yaml:"hostedZoneId"`
}

type SpecDistributionModulesIngressDNS struct {
	Private *SpecDistributionModulesIngressDNSPrivate `yaml:"private,omitempty"`
}

type SpecDistributionModulesIngressDNSPrivate struct {
	VpcId string `yaml:"vpcId"`
}

type SpecDistributionModulesIngressExternalDNS struct {
	PrivateIamRoleArn TypesAwsArn `yaml:"privateIamRoleArn"`
	PublicIamRoleArn  TypesAwsArn `yaml:"publicIamRoleArn"`
}

// --- tools configuration (opentofu/terraform S3 state) ---

type SpecToolsConfiguration struct {
	Opentofu  *SpecToolsConfigurationOpentofu  `yaml:"opentofu,omitempty"`
	Terraform *SpecToolsConfigurationTerraform `yaml:"terraform,omitempty"`
}

type SpecToolsConfigurationOpentofu struct {
	State SpecToolsConfigurationState `yaml:"state"`
}

type SpecToolsConfigurationTerraform struct {
	State SpecToolsConfigurationState `yaml:"state"`
}

type SpecToolsConfigurationState struct {
	S3 SpecToolsConfigurationTerraformStateS3 `yaml:"s3"`
}

type SpecToolsConfigurationTerraformStateS3 struct {
	BucketName           TypesAwsS3BucketName `yaml:"bucketName"`
	KeyPrefix            TypesAwsS3KeyPrefix  `yaml:"keyPrefix"`
	Region               TypesAwsRegion       `yaml:"region"`
	SkipRegionValidation *bool                `yaml:"skipRegionValidation,omitempty"`
}
