package template

type Templates struct {
	Includes        []string `yaml:"includes,omitempty"`
	Excludes        []string `yaml:"excludes,omitempty"`
	Suffix          string   `default:".tmpl" yaml:"suffix,omitempty"`
	ProcessFilename bool     `yaml:"processFilename,omitempty"`
}

type Config struct {
	Data    map[string]map[interface{}]interface{} `yaml:"data,omitempty"`
	Include map[string]string
	//Include Include `yaml:"include,omitempty"`
	Templates Templates `yaml:"templates,omitempty"`
}
