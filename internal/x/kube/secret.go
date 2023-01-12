// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kube

import (
	"encoding/base64"
	"fmt"
)

func CreateSecret(data []byte, name string, namespace string) string {
	secret := fmt.Sprintf(`{ 
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
			"name": "%s",
			"namespace": "%s"
		},
		"data": {
			"config": "%s"
		}
	}`, name, namespace, base64.StdEncoding.EncodeToString(data))

	return secret
}
