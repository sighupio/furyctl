package configuration

// AWS represents the configuration spec of a AWS bootstrap project including VPC and VPN
type AWS struct {
	Provisioner string `yaml:"provisioner"` // Required attribute

	NetworkCIDR         string   `yaml:"networkCIDR"`
	PublicSubnetsCIDRs  []string `yaml:"publicSubnetsCIDRs"`
	PrivateSubnetsCIDRs []string `yaml:"privateSubnetsCIDRs"`
	VPNSubnetCIDR       string   `yaml:"vpnSubnetCIDR"`
	VPNOperatorCIDRs    []string `yaml:"vpnOperatorCIDRs"`
	VPNSSHUsers         []string `yaml:"vpnSSHUsers"`
}
