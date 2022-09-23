package tools

type Tool interface {
	SrcPath() string
	Rename(basePath string) error
}

func NewFactory() *Factory {
	return &Factory{}
}

type Factory struct{}

func (f *Factory) Create(name, version string) Tool {
	if name == "furyagent" {
		return NewFuryAgent(version)
	}
	if name == "kubectl" {
		return NewKubectl(version)
	}
	if name == "kustomize" {
		return NewKustomize(version)
	}
	if name == "terraform" {
		return NewTerraform(version)
	}
	return nil
}
