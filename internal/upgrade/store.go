// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgrade

import (
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	errStateDataNotFound = errors.New("upgrade state data not found")
	errStateKeyNotFound  = errors.New("upgrade state key not found")
	// ErrStateNotFound means no upgrade state ConfigMap exists in the cluster.
	// Other read errors can identify an ongoing upgrade and must be returned.
	ErrStateNotFound = errors.New("upgrade state not found")
)

type PhaseStatus string

type Phase struct {
	Status PhaseStatus `yaml:"status"`
}

// Transition identifies the distribution upgrade that created an upgrade state.
// It remains available when the target configuration has already been persisted
// and there is no longer a configuration diff to derive it from.
type Transition struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// StagedWorkers records worker nodes skipped during an OnPremises upgrade.
type StagedWorkers struct {
	Nodes map[string]PhaseStatus `yaml:"nodes"`

	// ReadyForResume indicates that all other upgrade phases completed.
	ReadyForResume bool `yaml:"readyForResume"`
}

type Phases struct {
	PreInfrastructure  *Phase `yaml:"preInfrastructure,omitempty"`
	Infrastructure     *Phase `yaml:"infrastructure,omitempty"`
	PostInfrastructure *Phase `yaml:"postInfrastructure,omitempty"`
	PreKubernetes      *Phase `yaml:"preKubernetes,omitempty"`
	Kubernetes         *Phase `yaml:"kubernetes,omitempty"`
	PostKubernetes     *Phase `yaml:"postKubernetes,omitempty"`
	PreDistribution    *Phase `yaml:"preDistribution,omitempty"`
	Distribution       *Phase `yaml:"distribution,omitempty"`
	PostDistribution   *Phase `yaml:"postDistribution,omitempty"`
}

type State struct {
	Transition    *Transition    `yaml:"transition,omitempty"`
	StagedWorkers *StagedWorkers `yaml:"stagedWorkers,omitempty"`
	Phases        Phases         `yaml:"phases"`
}

func (s *State) HasStagedWorkers() bool {
	return s != nil && s.StagedWorkers != nil && len(s.StagedWorkers.Nodes) > 0
}

func (s *State) PendingStagedWorkers() []string {
	if !s.HasStagedWorkers() {
		return nil
	}

	nodes := make([]string, 0, len(s.StagedWorkers.Nodes))
	for node, status := range s.StagedWorkers.Nodes {
		if status == PhaseStatusPending || status == PhaseStatusFailed {
			nodes = append(nodes, node)
		}
	}

	slices.Sort(nodes)

	return nodes
}

func (s *State) AllStagedWorkersSucceeded() bool {
	if !s.HasStagedWorkers() {
		return false
	}

	for _, status := range s.StagedWorkers.Nodes {
		if status != PhaseStatusSuccess {
			return false
		}
	}

	return true
}

func (s *State) AllOnPremisesPhasesSucceeded() bool {
	if s == nil {
		return false
	}

	// Worker upgrades require every OnPremises phase to succeed.
	for _, phase := range []*Phase{
		s.Phases.PreKubernetes,
		s.Phases.Kubernetes,
		s.Phases.PostKubernetes,
		s.Phases.PreDistribution,
		s.Phases.Distribution,
		s.Phases.PostDistribution,
	} {
		if phase == nil || phase.Status != PhaseStatusSuccess {
			return false
		}
	}

	return true
}

func (s *State) MarkStagedWorker(node string, status PhaseStatus) {
	if !s.HasStagedWorkers() {
		return
	}

	s.StagedWorkers.Nodes[node] = status
}

type Storer interface {
	Store(state *State) error
	Get() ([]byte, error)
	Delete() error
	GetLatestResumablePhase(state *State) string
}

type StateStore struct {
	WorkDir       string
	KubectlRunner *kubectl.Runner
}

const (
	PhaseStatusSuccess PhaseStatus = "success"
	PhaseStatusFailed  PhaseStatus = "failed"
	PhaseStatusPending PhaseStatus = "pending"
)

func NewStateStore(workDir, kubectlVersion, binPath string) *StateStore {
	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl: path.Join(binPath, "kubectl", kubectlVersion, "kubectl"),
		WorkDir: workDir,
	}, true, true, false)

	return &StateStore{
		WorkDir:       workDir,
		KubectlRunner: runner,
	}
}

func (s *StateStore) Store(state *State) error {
	x, err := yamlx.MarshalV3(state)
	if err != nil {
		return fmt.Errorf("error while marshalling upgrade state: %w", err)
	}

	configMap, err := kubex.CreateConfigMap(x, "furyctl-upgrade-state", "state", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating configMap: %w", err)
	}

	cmPath := path.Join(s.WorkDir, "furyctl-upgrade-state.yaml")

	if err := iox.WriteFile(cmPath, configMap); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(cmPath)

	logrus.Info("Saving furyctl upgrade state file in the cluster...")

	if err := s.KubectlRunner.Apply(cmPath, "--force-conflicts"); err != nil {
		return fmt.Errorf("error while saving furyctl upgrade state file in the cluster: %w", err)
	}

	return nil
}

func (s *StateStore) Get() ([]byte, error) {
	configMap := map[string]any{}

	out, err := s.KubectlRunner.Get(
		true,
		"kube-system",
		"cm",
		"furyctl-upgrade-state",
		"-o",
		"yaml",
		"--ignore-not-found",
	)
	if err != nil {
		return nil, fmt.Errorf("error while getting current cluster upgrade state: %w", err)
	}

	if strings.TrimSpace(out) == "" {
		return nil, ErrStateNotFound
	}

	if err := yamlx.UnmarshalV3([]byte(out), configMap); err != nil {
		return nil, fmt.Errorf("error while unmarshalling current cluster upgrade state: %w", err)
	}

	data, ok := configMap["data"].(map[string]any)
	if !ok {
		return nil, errStateDataNotFound
	}

	configData, ok := data["state"].(string)
	if !ok {
		return nil, errStateKeyNotFound
	}

	return []byte(configData), nil
}

func (s *StateStore) Delete() error {
	if err := s.KubectlRunner.Delete("configmap", "furyctl-upgrade-state", "-n", "kube-system"); err != nil {
		return fmt.Errorf("error while deleting current cluster upgrade state: %w", err)
	}

	return nil
}

func (*StateStore) GetLatestResumablePhase(state *State) string {
	// Missing phases are skipped to resume ordinary upgrades.
	for _, phase := range cluster.GetPhasesOrder() {
		reflectedPhase := reflect.ValueOf(state.Phases).FieldByName(phase)

		if reflectedPhase.IsNil() {
			continue
		}

		phaseStatus := reflectedPhase.Elem().FieldByName("Status").String()

		if phaseStatus == string(PhaseStatusPending) || phaseStatus == string(PhaseStatusFailed) {
			return cluster.GetPhase(phase)
		}
	}

	return ""
}
