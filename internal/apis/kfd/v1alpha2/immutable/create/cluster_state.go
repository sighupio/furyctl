// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"
)

// clusterStateFile is the file that the preflight playbook writes. A distribution released
// before this file does not write it.
const clusterStateFile = "state.json"

var errClusterUnreachable = errors.New("furyctl cannot read the cluster")

// clusterState holds what the preflight playbook found on each host.
type clusterState struct {
	Reachable        []string `json:"reachable"`
	Unreachable      []string `json:"unreachable"`
	KubeletConfHosts []string `json:"kubeletConfHosts"`
	AdminConfHosts   []string `json:"adminConfHosts"`
}

// readClusterState reads the state file in dir. The second value is false when the file is not
// there, which is the case with a distribution released before the file.
func readClusterState(dir string) (clusterState, bool, error) {
	var state clusterState

	data, err := os.ReadFile(filepath.Join(dir, clusterStateFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, false, nil
		}

		return state, false, fmt.Errorf("error reading %s: %w", clusterStateFile, err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return state, false, fmt.Errorf("error parsing %s: %w", clusterStateFile, err)
	}

	return state, true, nil
}

// assess gives a warning for the operator, or the error that stops the run. The order of the
// cases is the logic: a host that holds admin.conf means a cluster that furyctl can read, and
// a host that holds kubelet.conf without one means a cluster that furyctl cannot read.
func (s clusterState) assess(controlPlane []string) (string, error) {
	switch {
	case len(s.AdminConfHosts) > 0:
		if len(s.Unreachable) == 0 {
			return "", nil
		}

		return fmt.Sprintf(
			"These hosts do not answer: %s. furyctl read the cluster from %s and continues. If these "+
				"hosts are new, or you provision them again, boot them while the assets server waits. If "+
				"they stay down, a phase that runs on them stops with an error.",
			strings.Join(s.Unreachable, ", "),
			s.AdminConfHosts[0],
		), nil

	case len(s.KubeletConfHosts) > 0:
		return "", s.unreachableError(controlPlane)

	case len(s.Reachable) == 0:
		return fmt.Sprintf(
			"furyctl could not read these hosts: %s. It continues as if the cluster does not exist. "+
				"A first apply gives this message, because the infrastructure phase creates those "+
				"hosts. If the cluster is there, stop furyctl and make sure that these hosts answer.",
			strings.Join(s.Unreachable, ", "),
		), nil

	case len(s.Unreachable) > 0:
		return fmt.Sprintf(
			"furyctl could not read these hosts: %s. The other hosts answer and hold no cluster, "+
				"so furyctl continues, and the infrastructure phase boots the hosts that do not answer.",
			strings.Join(s.Unreachable, ", "),
		), nil

	default:
		return "", nil
	}
}

// unreachableError tells the operator why a cluster that exists cannot be read. The run stops,
// and does not continue with a warning, because a new control plane would start a new cluster,
// and the kube-worker role does not join a host that holds kubelet.conf. The hosts that joined
// would stay with the old cluster, and nothing would report it. A host that does not answer is
// not a reason to stop on its own, because the infrastructure phase can boot it.
func (s clusterState) unreachableError(controlPlane []string) error {
	var down []string

	for _, h := range controlPlane {
		if slices.Contains(s.Unreachable, h) {
			down = append(down, h)
		}
	}

	if len(down) > 0 {
		return fmt.Errorf(
			"%w: these hosts joined a cluster: %s, and no control plane host gives its kubeconfig. "+
				"These control plane hosts do not answer: %s. Make sure that they answer, then apply "+
				"again. To make a new control plane, reset the hosts that joined first",
			errClusterUnreachable,
			strings.Join(s.KubeletConfHosts, ", "),
			strings.Join(down, ", "),
		)
	}

	return fmt.Errorf(
		"%w: these hosts joined a cluster: %s, and no control plane host holds admin.conf. "+
			"Restore the control plane, or reset the hosts that joined, then apply again",
		errClusterUnreachable,
		strings.Join(s.KubeletConfHosts, ", "),
	)
}

// summary gives one line that says what the preflight check found.
func (s clusterState) summary() string {
	total := len(s.Reachable) + len(s.Unreachable)

	line := fmt.Sprintf("Preflight state: %d of %d hosts answer, %d joined a cluster",
		len(s.Reachable), total, len(s.KubeletConfHosts))

	if len(s.AdminConfHosts) > 0 {
		line += ", admin.conf found on " + s.AdminConfHosts[0]
	}

	return line
}

// answers reports whether every host in hosts answered. A host that the playbook did not probe
// did not answer.
func (s clusterState) answers(hosts []string) bool {
	return lo.Every(s.Reachable, hosts)
}
