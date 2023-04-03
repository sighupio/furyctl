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

var logRegex = regexp.MustCompile(`'(.*?)'`)

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

func (c *Client) GetIngresses() ([]Ingress, error) {
	var result []Ingress

	log, err := c.kubeRunner.Get("all", "ingress", "-o",
		"jsonpath='[{range .items[*]}{\"{\"}\"Name\": \"{.metadata.name}\", "+
			"\"Host\": [{range .spec.rules[*]}\"{.host}\",{end}]{\"}\"},{end}]'")
	if err != nil {
		return result, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	logStringIndex := logRegex.FindStringIndex(log)

	if logStringIndex == nil {
		return result, nil
	}

	out := log[logStringIndex[0]+1 : logStringIndex[1]-1]

	out = strings.ReplaceAll(out, ",]", "]")

	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return result, fmt.Errorf("error while unmarshaling json: %w", err)
	}

	return result, nil
}

func (c *Client) GetPersistentVolumes() ([]string, error) {
	log, err := c.kubeRunner.Get("all", "pv", "-o", "jsonpath='{.items[*].metadata.name}'")
	if err != nil {
		return []string{}, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	logStringIndex := logRegex.FindStringIndex(log)

	if logStringIndex == nil {
		return []string{}, nil
	}

	return slices.Clean(strings.Split(log[logStringIndex[0]+1:logStringIndex[1]-1], " ")), nil
}

func (c *Client) GetListOfResourcesNs(ns, resName string) error {
	_, err := c.kubeRunner.Get(ns, resName, "-o",
		"jsonpath={range .items[*]}{\""+resName+" \"}\"{.metadata.name}\"{\" deleted (dry run)\"}{\"\\n\"}{end}")
	if err != nil {
		return fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	return nil
}

func (c *Client) GetLoadBalancers() ([]string, error) {
	log, err := c.kubeRunner.Get("all", "svc", "-o",
		"jsonpath='{.items[?(@.spec.type==\"LoadBalancer\")].metadata.name}'")
	if err != nil {
		return []string{}, fmt.Errorf("error while reading resources from cluster: %w", err)
	}

	logStringIndex := logRegex.FindStringIndex(log)

	if logStringIndex == nil {
		return []string{}, nil
	}

	return slices.Clean(strings.Split(log[logStringIndex[0]+1:logStringIndex[1]-1], " ")), nil
}

func (c *Client) DeleteAllResources(res, ns string) (string, error) {
	result, err := c.kubeRunner.DeleteAllResources(res, ns)
	if err != nil {
		return result, fmt.Errorf("error while deleting resources from cluster: %w", err)
	}

	return result, nil
}

func (c *Client) DeleteFromPath(path string, params ...string) (string, error) {
	result, err := c.kubeRunner.Delete(path, params...)
	if err != nil {
		return result, fmt.Errorf("error while deleting resources from cluster: %w", err)
	}

	return result, nil
}

func (c *Client) ToolVersion() (string, error) {
	version, err := c.kubeRunner.Version()
	if err != nil {
		return "", fmt.Errorf("error while getting tool version: %w", err)
	}

	return version, nil
}
