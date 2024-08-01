// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kfddistribution

import (
	"fmt"

	"github.com/sighupio/furyctl/internal/cluster"
)

type CertificatesRenewer struct{}

func (k *CertificatesRenewer) SetProperties(props []cluster.CertificatesRenewerProperty) {}

func (k *CertificatesRenewer) SetProperty(name string, value any) {}

func (k *CertificatesRenewer) Renew() error {
	return fmt.Errorf("you can't renew certificates for KFDDistribution")
}
