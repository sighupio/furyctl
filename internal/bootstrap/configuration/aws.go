package configuration

// AWS represents the configuration spec of a AWS bootstrap project including VPC and VPN
type AWS struct {
	Provisioner string `yaml:"provisioner"` // Required attribute

	NetworkCIDR         string   `yaml:"networkCIDR"`
	PublicSubnetsCIDRs  []string `yaml:"publicSubnetsCIDRs"`
	PrivateSubnetsCIDRs []string `yaml:"privateSubnetsCIDRs"`
	VPN                 AWSVPN   `yaml:"vpn"`
}

// AWSVPN represents an VPN configuration
type AWSVPN struct {
	Port          int      `yaml:"port"`
	InstanceType  string   `yaml:"instanceType"`
	DiskSize      int      `yaml:"diskSize"`
	OperatorName  string   `yaml:"operatorName"`
	DHParamsBits  int      `yaml:"dhParamsBits"`
	SubnetCIDR    string   `yaml:"subnetCIDR"`
	SSHUsers      []string `yaml:"sshUsers"`
	OperatorCIDRs []string `yaml:"operatorCIDRs"`
}
