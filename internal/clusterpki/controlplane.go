// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterpki

import (
	"github.com/sirupsen/logrus"
	pki "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
)

const (
	ControlPlaneSaKey     = "sa.key"
	ControlPlaneSaPub     = "sa.pub"
	ControlPlaneFProxyCrt = "front-proxy-ca.crt"
	ControlPlaneFProxyKey = "front-proxy-ca.key"
	ControlPlaneCaKey     = "ca.key"
	ControlPlaneCaCrt     = "ca.crt"
	ControlPlanePath      = "master"
	// We default to `master` as the folder name instead of controlplane because
	// that is where KFD's playbook template expects it.
	// See file fury-distribution/templates/kubernetes/onpremises/create-playbook.yaml.tpl at line 23.

)

// ControlPlanePKI implements the ClusterComponent interface.
type ControlPlanePKI struct {
	ClusterPKI
}

func (cp ControlPlanePKI) Create() error {
	// Create certificates for Kubernetes control plane.
	caCert, caKey, err := pki.NewCertificateAuthority(&cp.CertConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	saCert, saKey, err := pki.NewCertificateAuthority(&cp.CertConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	fpCert, fpKey, err := pki.NewCertificateAuthority(&cp.CertConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	certs := map[string][]byte{
		ControlPlaneCaCrt:     pki.EncodeCertPEM(caCert),
		ControlPlaneCaKey:     EncodePrivateKey(caKey),
		ControlPlaneSaPub:     pki.EncodeCertPEM(saCert),
		ControlPlaneSaKey:     EncodePrivateKey(saKey),
		ControlPlaneFProxyCrt: pki.EncodeCertPEM(fpCert),
		ControlPlaneFProxyKey: EncodePrivateKey(fpKey),
	}

	return cp.save(certs, ControlPlanePath)
}
