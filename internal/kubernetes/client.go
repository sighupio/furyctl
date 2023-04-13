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
	"github.com/sighupio/furyctl/internal/x/slices"
)

// cmdOutRegex is a regexp used to extract the output of the kubectl command, which is wrapped in single quotes.
var cmdOutRegex = regexp.MustCompile(`'(.*?)'`)

type Resource struct {
	Name string
	Kind string
}

type Ingress struct {
	Name string
	Host []string
}

type Client struct {
	kubeRunner *kubectl.Runner
}

func NewClient(
	binPath,
	workDir,
	kubeconfig string,
	serverSide,
	skipNotFound,
	clientVersion bool,
	executor execx.Executor,
) *Client {
	return &Client{
		kubeRunner: kubectl.NewRunner(
			executor,
			kubectl.Paths{
				Kubectl:    binPath,
				WorkDir:    workDir,
				Kubeconfig: kubeconfig,
			},
			serverSide,
			skipNotFound,
			clientVersion,
		),
	}
}

// GetIngresses returns a list of ingresses in the cluster, this is done by using the jsonpath format option
// of kubectl to get a valid json output to be unmarshalled.
func (c *Client) GetIngresses() ([]Ingress, error) {
	var result []Ingress

	cmdOut, err := c.kubeRunner.Get("all", "ingress", "-o",
		"jsonpath='[{range .items[*]}{\"{\"}\"Name\": \"{.metadata.name}\", "+
			"\"Host\": [{range .spec.rules[*]}\"{.host}\",{end}]{\"}\"},{end}]'")
	if err != nil {
		return result, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	idx := cmdOutRegex.FindStringIndex(cmdOut)
	if idx == nil {
		return result, nil
	}

	out := cmdOut[idx[0]+1 : idx[1]-1]

	out = strings.ReplaceAll(out, ",]", "]")

	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return result, fmt.Errorf("error while unmarshaling json: %w", err)
	}

	return result, nil
}

func (c *Client) GetPersistentVolumes() ([]string, error) {
	cmdOut, err := c.kubeRunner.Get("all", "pv", "-o", "jsonpath='{.items[*].metadata.name}'")
	if err != nil {
		return []string{}, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	idx := cmdOutRegex.FindStringIndex(cmdOut)
	if idx == nil {
		return []string{}, nil
	}

	return slices.Clean(strings.Split(cmdOut[idx[0]+1:idx[1]-1], " ")), nil
}

func (c *Client) ListNamespaceResources(resName, ns string) ([]Resource, error) {
	var result []Resource

	cmdOut, err := c.kubeRunner.Get(ns, resName, "-o",
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

func (c *Client) GetLoadBalancers() ([]string, error) {
	cmdOut, err := c.kubeRunner.Get("all", "svc", "-o",
		"jsonpath='{.items[?(@.spec.type==\"LoadBalancer\")].metadata.name}'")
	if err != nil {
		return []string{}, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	idx := cmdOutRegex.FindStringIndex(cmdOut)
	if idx == nil {
		return []string{}, nil
	}

	return slices.Clean(strings.Split(cmdOut[idx[0]+1:idx[1]-1], " ")), nil
}

func (c *Client) DeleteAllResources(res, ns string) (string, error) {
	cmdOut, err := c.kubeRunner.DeleteAllResources(ns, res)
	if err != nil {
		return cmdOut, fmt.Errorf("error while deleting resources from cluster: %w", err)
	}

	return cmdOut, nil
}

func (c *Client) DeleteResource(name, res, ns string) (string, error) {
	cmdOut, err := c.kubeRunner.DeleteResource(ns, res, name)
	if err != nil {
		return cmdOut, fmt.Errorf("error while deleting resource from cluster: %w", err)
	}

	return cmdOut, nil
}

func (c *Client) DeleteFromPath(path string, params ...string) (string, error) {
	cmdOut, err := c.kubeRunner.Delete(path, params...)
	if err != nil {
		return cmdOut, fmt.Errorf("error while deleting resources from cluster: %w", err)
	}

	return cmdOut, nil
}

func (c *Client) ToolVersion() (string, error) {
	cmdOut, err := c.kubeRunner.Version()
	if err != nil {
		return "", fmt.Errorf("error while getting tool version: %w", err)
	}

	return cmdOut, nil
}
