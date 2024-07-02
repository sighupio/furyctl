// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legacy

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
)

var ErrNoLabelFound = errors.New("no label found")

type ProviderKind map[string][]RegistrySpec

func (k *ProviderKind) getLabeledURI(providerName, label string) (string, error) {
	for name, providerSpecList := range *k {
		if name != providerName {
			continue
		}

		for _, providerMap := range providerSpecList {
			if providerMap.Label != label {
				continue
			}

			return "git::" + providerMap.BaseURI, nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrNoLabelFound, label)
}

func (k *ProviderKind) pickCloudProviderURL(cloudProvider ProviderOptSpec) string {
	url, err := k.getLabeledURI(cloudProvider.Name, cloudProvider.Label)
	if err != nil {
		logrus.Fatal(err)
	}

	return url
}
