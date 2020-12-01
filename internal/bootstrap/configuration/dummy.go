package configuration

// Dummy represents the configuration spec of a simple dummy project
type Dummy struct {
	Provisioner string `yaml:"provisioner"` // Required attribute

	RSABits int `yaml:"rsaBits"`
}
