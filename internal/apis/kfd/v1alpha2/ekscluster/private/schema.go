// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package private contains furyctl's curated, hand-maintained view of the
// EKSCluster furyctl.yaml (apiVersion kfd.sighup.io/v1alpha2).
//
// It is called "private" for historical reasons: besides the fields furyctl
// reads from the user's furyctl.yaml, it also models the EKS-internal fields
// that furyctl itself fills in by injecting infrastructure (Terraform/OpenTofu)
// outputs before rendering templates — see phases/distribution.go
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
	Kind     Kind     `yaml:"kind"     json:"kind"`
	Metadata Metadata `yaml:"metadata" json:"metadata"`
	Spec     Spec     `yaml:"spec"     json:"spec"`
}

type Metadata struct {
	Name string `yaml:"name" json:"name"`
}

type Spec struct {
	Region             TypesAwsRegion         `yaml:"region"                   json:"region"`
	Infrastructure     *SpecInfrastructure    `yaml:"infrastructure,omitempty" json:"infrastructure,omitempty"`
	Kubernetes         SpecKubernetes         `yaml:"kubernetes"               json:"kubernetes"`
	Distribution       SpecDistribution       `yaml:"distribution"             json:"distribution"`
	ToolsConfiguration SpecToolsConfiguration `yaml:"toolsConfiguration"       json:"toolsConfiguration"`
}

// --- infrastructure ---

type SpecInfrastructure struct {
	Vpc *SpecInfrastructureVpc `yaml:"vpc,omitempty" json:"vpc,omitempty"`
	Vpn *SpecInfrastructureVpn `yaml:"vpn,omitempty" json:"vpn,omitempty"`
}

type SpecInfrastructureVpc struct {
	Network SpecInfrastructureVpcNetwork `yaml:"network" json:"network"`
}

type SpecInfrastructureVpcNetwork struct {
	Cidr TypesCidr `yaml:"cidr" json:"cidr"`
}

// TypesTcpPort is a TCP port number.
type TypesTcpPort int

type SpecInfrastructureVpn struct {
	BucketNamePrefix    *string       `yaml:"bucketNamePrefix,omitempty"    json:"bucketNamePrefix,omitempty"`
	IamUserNameOverride *string       `yaml:"iamUserNameOverride,omitempty" json:"iamUserNameOverride,omitempty"`
	Instances           *int          `yaml:"instances,omitempty"           json:"instances,omitempty"`
	OperatorName        *string       `yaml:"operatorName,omitempty"        json:"operatorName,omitempty"`
	Port                *TypesTcpPort `yaml:"port,omitempty"                json:"port,omitempty"`
}

// IsConfigured reports whether a VPN is requested. A nil receiver means no VPN
// block; nil Instances defaults to one; otherwise configured when > 0.
func (v *SpecInfrastructureVpn) IsConfigured() bool {
	if v == nil {
		return false
	}

	return v.Instances == nil || *v.Instances > 0
}

// --- kubernetes ---

type SpecKubernetes struct {
	ApiServer                        SpecKubernetesAPIServer  `yaml:"apiServer"                                  json:"apiServer"`
	ClusterIAMRoleNamePrefixOverride *string                  `yaml:"clusterIAMRoleNamePrefixOverride,omitempty" json:"clusterIAMRoleNamePrefixOverride,omitempty"`
	NodePools                        []SpecKubernetesNodePool `yaml:"nodePools"                                  json:"nodePools"`
	SubnetIds                        []TypesAwsSubnetId       `yaml:"subnetIds,omitempty"                        json:"subnetIds,omitempty"`
	VpcId                            *TypesAwsVpcId           `yaml:"vpcId,omitempty"                            json:"vpcId,omitempty"`
	WorkersIAMRoleNamePrefixOverride *string                  `yaml:"workersIAMRoleNamePrefixOverride,omitempty" json:"workersIAMRoleNamePrefixOverride,omitempty"`
}

type SpecKubernetesAPIServer struct {
	PrivateAccess bool `yaml:"privateAccess" json:"privateAccess"`
	PublicAccess  bool `yaml:"publicAccess"  json:"publicAccess"`
}

type SpecKubernetesNodePool struct {
	Size SpecKubernetesNodePoolSize `yaml:"size" json:"size"`
}

type SpecKubernetesNodePoolSize struct {
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
}

// --- distribution: read (Dr.Type, Aws overrides) + injection targets ---

type SpecDistribution struct {
	Modules SpecDistributionModules `yaml:"modules" json:"modules"`
}

type SpecDistributionModules struct {
	Aws     *SpecDistributionModulesAws    `yaml:"aws,omitempty" json:"aws,omitempty"`
	Dr      SpecDistributionModulesDr      `yaml:"dr"            json:"dr"`
	Ingress SpecDistributionModulesIngress `yaml:"ingress"       json:"ingress"`
}

type SpecDistributionModulesAws struct {
	ClusterAutoscaler      SpecDistributionModulesAwsClusterAutoscaler      `yaml:"clusterAutoscaler"      json:"clusterAutoscaler"`
	EbsCsiDriver           SpecDistributionModulesAwsEbsCsiDriver           `yaml:"ebsCsiDriver"           json:"ebsCsiDriver"`
	LoadBalancerController SpecDistributionModulesAwsLoadBalancerController `yaml:"loadBalancerController" json:"loadBalancerController"`
}

type SpecDistributionModulesAwsEbsCsiDriver struct {
	IamRoleArn TypesAwsArn                        `yaml:"iamRoleArn"          json:"iamRoleArn"`
	Overrides  *ComponentOverridesWithIAMRoleName `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

type SpecDistributionModulesAwsClusterAutoscaler struct {
	IamRoleArn TypesAwsArn                        `yaml:"iamRoleArn"          json:"iamRoleArn"`
	Overrides  *ComponentOverridesWithIAMRoleName `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

type SpecDistributionModulesAwsLoadBalancerController struct {
	IamRoleArn TypesAwsArn                        `yaml:"iamRoleArn"          json:"iamRoleArn"`
	Overrides  *ComponentOverridesWithIAMRoleName `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

type ComponentOverridesWithIAMRoleName struct {
	IamRoleName *string `yaml:"iamRoleName,omitempty" json:"iamRoleName,omitempty"`
}

type SpecDistributionModulesDr struct {
	Type   string                           `yaml:"type"             json:"type"`
	Velero *SpecDistributionModulesDrVelero `yaml:"velero,omitempty" json:"velero,omitempty"`
}

type SpecDistributionModulesDrVelero struct {
	Eks SpecDistributionModulesDrVeleroEks `yaml:"eks" json:"eks"`
}

type SpecDistributionModulesDrVeleroEks struct {
	IamRoleArn TypesAwsArn `yaml:"iamRoleArn" json:"iamRoleArn"`
}

type SpecDistributionModulesIngress struct {
	CertManager SpecDistributionModulesIngressCertManager `yaml:"certManager"   json:"certManager"`
	Dns         *SpecDistributionModulesIngressDNS        `yaml:"dns,omitempty" json:"dns,omitempty"`
	ExternalDns SpecDistributionModulesIngressExternalDNS `yaml:"externalDns"   json:"externalDns"`
}

type SpecDistributionModulesIngressCertManager struct {
	ClusterIssuer SpecDistributionModulesIngressCertManagerClusterIssuer `yaml:"clusterIssuer" json:"clusterIssuer"`
}

type SpecDistributionModulesIngressCertManagerClusterIssuer struct {
	Route53 SpecDistributionModulesIngressClusterIssuerRoute53 `yaml:"route53" json:"route53"`
}

type SpecDistributionModulesIngressClusterIssuerRoute53 struct {
	IamRoleArn   TypesAwsArn `yaml:"iamRoleArn"   json:"iamRoleArn"`
	HostedZoneId string      `yaml:"hostedZoneId" json:"hostedZoneId"`
}

type SpecDistributionModulesIngressDNS struct {
	Private *SpecDistributionModulesIngressDNSPrivate `yaml:"private,omitempty" json:"private,omitempty"`
}

type SpecDistributionModulesIngressDNSPrivate struct {
	VpcId string `yaml:"vpcId" json:"vpcId"`
}

type SpecDistributionModulesIngressExternalDNS struct {
	PrivateIamRoleArn TypesAwsArn `yaml:"privateIamRoleArn" json:"privateIamRoleArn"`
	PublicIamRoleArn  TypesAwsArn `yaml:"publicIamRoleArn"  json:"publicIamRoleArn"`
}

// --- tools configuration (opentofu/terraform S3 state) ---

type SpecToolsConfiguration struct {
	Opentofu  *SpecToolsConfigurationOpentofu  `yaml:"opentofu,omitempty"  json:"opentofu,omitempty"`
	Terraform *SpecToolsConfigurationTerraform `yaml:"terraform,omitempty" json:"terraform,omitempty"`
}

type SpecToolsConfigurationOpentofu struct {
	State SpecToolsConfigurationState `yaml:"state" json:"state"`
}

type SpecToolsConfigurationTerraform struct {
	State SpecToolsConfigurationState `yaml:"state" json:"state"`
}

type SpecToolsConfigurationState struct {
	S3 SpecToolsConfigurationTerraformStateS3 `yaml:"s3" json:"s3"`
}

type SpecToolsConfigurationTerraformStateS3 struct {
	BucketName           TypesAwsS3BucketName `yaml:"bucketName"                     json:"bucketName"`
	KeyPrefix            TypesAwsS3KeyPrefix  `yaml:"keyPrefix"                      json:"keyPrefix"`
	Region               TypesAwsRegion       `yaml:"region"                         json:"region"`
	SkipRegionValidation *bool                `yaml:"skipRegionValidation,omitempty" json:"skipRegionValidation,omitempty"`
}
