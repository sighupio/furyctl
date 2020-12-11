package configuration

// AWSSimple represents the configuration spec of a simple AWS K8S cluster
type AWSSimple struct {
	Region             string   `yaml:"region"`
	Version            string   `yaml:"version"`
	PublicSubnetID     string   `yaml:"publicSubnetID"`
	PrivateSubnetID    string   `yaml:"privateSubnetID"`
	TrustedCIDRs       []string `yaml:"trustedCIDRs"`
	MasterInstanceType string   `yaml:"masterInstanceType"`
	WorkerInstanceType string   `yaml:"workerInstanceType"`
	WorkerCount        int      `yaml:"workerCount"`
	PodNetworkCIDR     string   `yaml:"podNetworkCIDR"`
}
