package onpremises

import (
	"errors"

	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var ErrAnsibleNotInstalled = errors.New("ansible is not installed")

type ExtraToolsValidator struct {
	executor execx.Executor
}

func (x *ExtraToolsValidator) Validate(_ string) ([]string, []error) {
	var (
		oks  []string
		errs []error
	)

	if err := x.ansible(); err != nil {
		errs = append(errs, err)
	} else {
		oks = append(oks, "ansible")
	}

	return oks, errs
}

func (x *ExtraToolsValidator) ansible() error {
	ansibleRunner := ansible.NewRunner(x.executor, ansible.Paths{
		Ansible: "ansible",
	})

	if _, err := ansibleRunner.Version(); err != nil {
		return ErrAnsibleNotInstalled
	}

	return nil
}

func NewExtraToolsValidator(executor execx.Executor) *ExtraToolsValidator {
	return &ExtraToolsValidator{
		executor: executor,
	}
}
