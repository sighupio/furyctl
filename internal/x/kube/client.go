// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kube

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	kubeClient *kubernetes.Clientset
}

func NewClient(config *rest.Config) (*Client, error) {
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		kubeClient: c,
	}, nil
}

func (c *Client) StoreDataAsSecret(data []byte, name string, namespace string) error {
	if _, err := c.kubeClient.CoreV1().Secrets(namespace).Create(
		context.Background(),
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Data: map[string][]byte{
				"data": data,
			},
		}, metav1.CreateOptions{}); err != nil {

		return err

	}

	return nil
}
