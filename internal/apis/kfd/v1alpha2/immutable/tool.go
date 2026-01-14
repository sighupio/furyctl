// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"

	"github.com/sighupio/furyctl/internal/cluster"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	ErrToolNotFound    = errors.New("required tool not found")
	ErrToolUnsupported = errors.New("tool version not supported")
)

// ToolValidator validates required tools for Immutable kind operations.
type ToolValidator struct {
	binPath string
}

// NewToolValidator creates a new tool validator.
func NewToolValidator(binPath string) *ToolValidator {
	return &ToolValidator{
		binPath: binPath,
	}
}

// Validate checks that all required tools are available and have correct versions.
func (tv *ToolValidator) Validate(phase string) error {
	requiredTools := tv.getRequiredTools(phase)

	for _, tool := range requiredTools {
		if err := tv.validateTool(tool); err != nil {
			return fmt.Errorf("tool validation failed for %s: %w", tool, err)
		}
	}

	return nil
}

func (*ToolValidator) getRequiredTools(phase string) []string {
	switch phase {
	case cluster.OperationPhaseInfrastructure:
		return []string{"butane"} // For Butane to Ignition conversion.

	case cluster.OperationPhaseKubernetes:
		return []string{"ansible-playbook", "ssh"}

	case cluster.OperationPhaseDistribution:
		return []string{"kubectl", "kustomize", "helm"}

	default:
		return []string{}
	}
}

func (*ToolValidator) validateTool(toolName string) error {
	executor := execx.NewStdExecutor()

	// Check if tool exists.
	cmd := executor.Command("which", toolName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrToolNotFound, toolName)
	}

	return nil
}
