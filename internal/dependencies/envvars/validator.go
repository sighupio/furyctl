package envvars

import (
	"errors"
	"fmt"
	"os"
)

var ErrMissingEnvVar = errors.New("missing environment variable")

func NewValidator() *Validator {
	return &Validator{}
}

type Validator struct{}

func (ev *Validator) Validate(kind string) []error {
	if kind == "EKSCluster" {
		return ev.checkEKSCluster()
	}

	return nil
}

func (ev *Validator) checkEKSCluster() []error {
	errs := make([]error, 0)

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		errs = append(errs, fmt.Errorf("%w: AWS_ACCESS_KEY_ID", ErrMissingEnvVar))
	}

	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		errs = append(errs, fmt.Errorf("%w: AWS_SECRET_ACCESS_KEY", ErrMissingEnvVar))
	}

	if os.Getenv("AWS_DEFAULT_REGION") == "" {
		errs = append(errs, fmt.Errorf("%w: AWS_DEFAULT_REGION", ErrMissingEnvVar))
	}

	return errs
}
