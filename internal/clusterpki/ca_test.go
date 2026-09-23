// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package clusterpki_test

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certutil "k8s.io/client-go/util/cert"

	"github.com/sighupio/furyctl/internal/clusterpki"
)

// These lock the shape of the CA that kubeadm's pkiutil used to produce, so a
// later refactor cannot quietly change what lands in a user's PKI folder.
func TestNewCertificateAuthority(t *testing.T) {
	t.Parallel()

	cfg := certutil.Config{
		CommonName:   "SIGHUP s.r.l. Server",
		Organization: []string{"SIGHUP s.r.l."},
	}

	cert, key, err := clusterpki.NewCertificateAuthority(&cfg)
	require.NoError(t, err)
	require.NotNil(t, cert)
	require.NotNil(t, key)

	assert.True(t, cert.IsCA)
	assert.True(t, cert.BasicConstraintsValid)
	assert.Equal(t, "SIGHUP s.r.l. Server", cert.Subject.CommonName)
	assert.Equal(t, []string{"SIGHUP s.r.l."}, cert.Subject.Organization)

	// kubeadm repeats the CN as a SAN; existing PKI folders rely on it.
	assert.Equal(t, []string{"SIGHUP s.r.l. Server"}, cert.DNSNames)

	assert.Equal(
		t,
		x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign,
		cert.KeyUsage,
	)

	rsaKey, ok := key.(*rsa.PrivateKey)
	require.True(t, ok, "expected an RSA key, got %T", key)
	assert.Equal(t, 2048, rsaKey.N.BitLen())

	assert.Positive(t, cert.SerialNumber.Sign(), "serial must be >= 1")

	// Ten years, give or take a leap day.
	validity := cert.NotAfter.Sub(cert.NotBefore)
	assert.InDelta(t, (10 * 365 * 24 * time.Hour).Hours(), validity.Hours(), 24)

	// It must actually be self-signed, not merely marked as a CA.
	require.NoError(t, cert.CheckSignatureFrom(cert))
}

func TestNewCertificateAuthorityEmptyCommonName(t *testing.T) {
	t.Parallel()

	cert, _, err := clusterpki.NewCertificateAuthority(&certutil.Config{})
	require.NoError(t, err)

	assert.Empty(t, cert.DNSNames, "an empty CommonName must not become a SAN")
}

func TestNewCertificateAuthorityHonoursNotBefore(t *testing.T) {
	t.Parallel()

	notBefore := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)

	cert, _, err := clusterpki.NewCertificateAuthority(&certutil.Config{NotBefore: notBefore})
	require.NoError(t, err)

	assert.Equal(t, notBefore, cert.NotBefore.UTC())
}

func TestEncodeCertPEM(t *testing.T) {
	t.Parallel()

	cert, _, err := clusterpki.NewCertificateAuthority(&certutil.Config{CommonName: "test-ca"})
	require.NoError(t, err)

	block, rest := pem.Decode(clusterpki.EncodeCertPEM(cert))
	require.NotNil(t, block)

	assert.Empty(t, rest)
	assert.Equal(t, "CERTIFICATE", block.Type)
	assert.Equal(t, cert.Raw, block.Bytes)
}
