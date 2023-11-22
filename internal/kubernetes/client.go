// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubernetes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

// cmdOutRegex is a regexp used to extract the output of the kubectl command, which is wrapped in single quotes.
var cmdOutRegex = regexp.MustCompile(`'(.*?)'`)

type Resource struct {
	Name string
	Kind string
}

type Client struct {
	kubeRunner *kubectl.Runner
}

func NewClient(
	binPath,
	workDir string,
	serverSide,
	skipNotFound,
	clientVersion bool,
	executor execx.Executor,
) *Client {
	return &Client{
		kubeRunner: kubectl.NewRunner(
			executor,
			kubectl.Paths{
				Kubectl: binPath,
				WorkDir: workDir,
			},
			serverSide,
			skipNotFound,
			clientVersion,
		),
	}
}

func (c *Client) ListNamespaceResources(resName, ns string) ([]Resource, error) {
	var result []Resource

	cmdOut, err := c.kubeRunner.Get(false, ns, resName, "-o",
		`jsonpath='[{range .items[*]}{"{"}"Name": "{.metadata.name}", "Kind": "{.kind}"{"}"},{end}]'`)
	if err != nil {
		return result, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	idx := cmdOutRegex.FindStringIndex(cmdOut)
	if idx == nil {
		return result, nil
	}

	out := cmdOut[idx[0]+1 : idx[1]-1]

	out = strings.ReplaceAll(out, ",]", "]")

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return result, fmt.Errorf("error while unmarshaling json: %w", err)
	}

	return result, nil
}

func (c *Client) ToolVersion() (string, error) {
	cmdOut, err := c.kubeRunner.Version()
	if err != nil {
		return "", fmt.Errorf("error while getting tool version: %w", err)
	}

	return cmdOut, nil
}
