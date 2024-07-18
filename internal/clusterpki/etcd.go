// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterpki

import (
	"fmt"

	pki "k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
)

const (
	EtcdCaCrt = "ca.crt"
	EtcdCaKey = "ca.key"
	etcdPath  = "etcd"
)

// Etcd implements the ClusterComponent Interface.
type Etcd struct {
	ClusterPKI
}

func (e Etcd) Create() error {
	ca, privateKey, err := pki.NewCertificateAuthority(&e.CertConfig)
	if err != nil {
		return fmt.Errorf("error while creating CA for etcd: %w", err)
	}

	certs := map[string][]byte{
		EtcdCaCrt: pki.EncodeCertPEM(ca),
		EtcdCaKey: EncodePrivateKey(privateKey),
	}

	return e.save(certs, etcdPath)
}
