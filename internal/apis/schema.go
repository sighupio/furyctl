package apis

import "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks"

type ExtraSchemaValidator interface {
	Validate(confPath string) error
}

func NewExtraSchemaValidatorFactory(apiVersion, kind string) ExtraSchemaValidator {
	switch apiVersion {
	case "kfd.sighup.io/v1alpha2":
		switch kind {
		case "EKSCluster":
			return &eks.ExtraSchemaValidator{}

		default:
			return nil
		}

	default:
		return nil
	}
}
