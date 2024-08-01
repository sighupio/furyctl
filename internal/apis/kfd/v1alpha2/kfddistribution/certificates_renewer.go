// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kfddistribution

import (
	"errors"

	"github.com/sighupio/furyctl/internal/cluster"
)

var ErrRenewNotSupported = errors.New("you can't renew certificates for KFDDistribution")

type CertificatesRenewer struct{}

func (*CertificatesRenewer) SetProperties(_ []cluster.CertificatesRenewerProperty) {}

func (*CertificatesRenewer) SetProperty(_ string, _ any) {}

func (*CertificatesRenewer) Renew() error {
	return ErrRenewNotSupported
}
