// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package secrets creates the random secrets that the configuration file of a cluster needs, and
// writes each one to its own file. The configuration file then reads the values with the
// `{file://<path>}` notation.
package secrets

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/distribution"
)

const (
	// The number of random bytes of a key. Pomerium reads the cookie secret and the shared secret
	// as 256-bit keys, thus 32 bytes.
	keyLength = 32

	// The number of characters of the name of a key of the encryption at rest. The name tells one
	// key from another, thus a few characters are enough.
	keyNameLength = 8

	// The number of characters of the Keepalived passphrase. The vrrpd protocol holds 8
	// characters, and the schema rejects a longer value.
	passphraseLength = 8

	// The permissions of the folders that hold the secrets.
	folderPermissions = 0o700

	// The permissions of the files that hold the secrets.
	filePermissions = 0o600

	// The permissions of a folder or of a file that give access to the group and to the others.
	otherPermissions = 0o077

	// EncryptionField is the field that holds the encryption at rest configuration of etcd. The
	// command names it in the note about a new key: see the create command.
	EncryptionField = etcdEncryptionConfiguration

	// EncryptionComponent is the name of the component that encrypts the secrets of etcd at rest.
	// Only the user can ask for it: see Encryption.
	EncryptionComponent = "etcd-encryption"
)

var (
	// ErrUnknownComponent reports a component name that furyctl does not create secrets for.
	ErrUnknownComponent = errors.New("unknown component")

	// ErrFileHoldsNoSecret reports a path that furyctl cannot read a secret from: a folder, or a
	// file with no value in it. The `{file://}` notation of the configuration file then gives an
	// empty value to the cluster.
	ErrFileHoldsNoSecret = errors.New("the path holds no secret")
)

// Secret is one random value that furyctl creates for one field of the configuration file.
type Secret struct {
	// Name is the name of the field of the configuration file. The file of the value has that name
	// too.
	Name string

	Path string

	// PathByKind holds the field for the kinds that keep it at another place. Select gives Path the
	// value for the kind of the configuration file.
	PathByKind map[string]string

	New func() (string, error)
}

// Component is a package or a service that needs secrets.
type Component struct {
	// Name is the name of the component. It is the name on the command line, and the name of the
	// folder that holds the secrets of the component.
	Name string

	Secrets []Secret

	// Needed reports whether a cluster with this configuration uses the component. It is nil for a
	// component that only the user can ask for, because the configuration file says nothing about
	// it: such a component needs its name on the command line, or the question that the command
	// asks. A run that names no component never creates its secrets.
	Needed func(*Config) bool
}

// Result reports the file of one secret.
type Result struct {
	Path string

	File string

	// Created is false when the file was already there: furyctl then kept the value of that file.
	Created bool
}

// All returns each component that furyctl creates secrets for, in the order that it writes them.
func All() []Component {
	return []Component{
		{
			Name:   "pomerium",
			Needed: func(c *Config) bool { return c.String(authProviderType) == "sso" },
			Secrets: []Secret{
				{
					Name: "COOKIE_SECRET",
					Path: "spec.distribution.modules.auth.pomerium.secrets.COOKIE_SECRET",
					New:  key,
				},
				{
					Name: "IDP_CLIENT_SECRET",
					Path: "spec.distribution.modules.auth.pomerium.secrets.IDP_CLIENT_SECRET",
					New:  password,
				},
				{
					Name: "SHARED_SECRET",
					Path: "spec.distribution.modules.auth.pomerium.secrets.SHARED_SECRET",
					New:  key,
				},
				{
					Name: "SIGNING_KEY",
					Path: "spec.distribution.modules.auth.pomerium.secrets.SIGNING_KEY",
					New:  signingKey,
				},
			},
		},
		{
			Name:   "gangplank",
			Needed: func(c *Config) bool { return c.Bool(oidcKubernetesAuthEnabled) },
			Secrets: []Secret{
				{
					Name: "clientSecret",
					Path: "spec.distribution.modules.auth.oidcKubernetesAuth.clientSecret",
					New:  password,
				},
				{
					Name: "sessionSecurityKey",
					Path: "spec.distribution.modules.auth.oidcKubernetesAuth.sessionSecurityKey",
					New:  key,
				},
			},
		},
		{
			Name:   "basic-auth",
			Needed: func(c *Config) bool { return c.String(authProviderType) == "basicAuth" },
			Secrets: []Secret{
				{
					Name: "password",
					Path: "spec.distribution.modules.auth.provider.basicAuth.password",
					New:  password,
				},
			},
		},
		minio("logging", func(c *Config) bool {
			// The logging module deploys MinIO with both of its engines. The backend of the engine
			// makes no difference. An absent type is the default, opensearch.
			return slices.Contains([]string{"", "opensearch", "loki"}, c.String(moduleType("logging")))
		}),
		minio("monitoring", func(c *Config) bool {
			// Only Mimir writes to MinIO. The default type, prometheus, writes to its own volumes.
			return c.String(moduleType("monitoring")) == "mimir" &&
				writesToMinIO(c, mimirBackend)
		}),
		minio("tracing", func(c *Config) bool {
			return c.String(moduleType("tracing")) != "none" &&
				writesToMinIO(c, tempoBackend)
		}),
		{
			Name: "keepalived",
			Needed: func(c *Config) bool {
				return c.Bool(onPremisesLoadBalancersKeepalivedEnabled) || c.Bool(immutableLoadBalancersKeepalivedEnabled)
			},
			Secrets: []Secret{
				{
					Name: "passphrase",
					Path: "spec.kubernetes.loadBalancers.keepalived.passphrase",
					PathByKind: map[string]string{
						distribution.ImmutableKind: "spec.infrastructure.loadBalancers.keepalived.passphrase",
					},
					New: passphrase,
				},
			},
		},
		{
			// The Immutable kind can give the control plane nodes a virtual IP, and that Keepalived
			// cluster needs a passphrase of its own: the schema asks for a unique one for each
			// Keepalived cluster.
			Name:   "keepalived-controlplane",
			Needed: func(c *Config) bool { return c.Bool(immutableControlPlaneKeepalivedEnabled) },
			Secrets: []Secret{
				{
					Name: "passphrase",
					Path: "spec.kubernetes.controlPlane.keepalived.passphrase",
					New:  passphrase,
				},
			},
		},
		{
			Name: EncryptionComponent,
			// Needed is nil on purpose: the configuration file says nothing about the encryption at
			// rest, because the presence of the field is the switch. The command asks the user, see
			// Encryption.
			Secrets: []Secret{
				{
					Name: "configuration",
					Path: etcdEncryptionConfiguration,
					New:  encryptionConfiguration,
				},
			},
		},
		{
			Name:   "haproxy-stats",
			Needed: func(c *Config) bool { return c.Bool(onPremisesLoadBalancersEnabled) },
			Secrets: []Secret{
				{
					Name: "password",
					Path: "spec.kubernetes.loadBalancers.stats.password",
					New:  password,
				},
			},
		},
	}
}

// minio returns the component of the MinIO deployment of a module. Each module deploys its own
// MinIO, and the password of the root user must be different between them.
func minio(module string, needed func(*Config) bool) Component {
	return Component{
		Name:   "minio-" + module,
		Needed: needed,
		Secrets: []Secret{
			{
				Name: "password",
				Path: "spec.distribution.modules." + module + ".minio.rootUser.password",
				New:  password,
			},
		},
	}
}

// moduleType returns the field that tells which engine a module deploys.
func moduleType(module string) string {
	return "spec.distribution.modules." + module + ".type"
}

// writesToMinIO reports whether the engine of a module writes to the MinIO of the module. MinIO is
// the default backend, thus a field with no value is a MinIO backend.
func writesToMinIO(c *Config, backendPath string) bool {
	backend := c.String(backendPath)

	return backend == "" || backend == "minio"
}

// Select returns the components with the given names. No name returns the components that the
// configuration uses, or every component when there is no configuration file.
func Select(names []string, cfg *Config) ([]Component, error) {
	all := All()

	if len(names) > 0 {
		// A name twice would write the same files twice, and the report would tell that furyctl
		// kept a file that it created one moment before.
		selected := make([]Component, 0, len(names))

		for _, name := range lo.Uniq(names) {
			component, found := lo.Find(all, func(c Component) bool { return c.Name == name })
			if !found {
				return nil, fmt.Errorf("%w %q: furyctl creates the secrets of %s",
					ErrUnknownComponent, name, strings.Join(Names(), ", "))
			}

			selected = append(selected, component)
		}

		return resolve(selected, cfg), nil
	}

	if cfg == nil {
		return resolve(lo.Filter(all, func(c Component, _ int) bool { return c.Needed != nil }), cfg), nil
	}

	return resolve(lo.Filter(all, func(c Component, _ int) bool {
		return c.Needed != nil && c.Needed(cfg)
	}), cfg), nil
}

// resolve returns the components with the field that each secret has for this kind. The Immutable
// kind keeps the load balancers in the infrastructure phase, thus its fields are not the fields of
// the other kinds.
func resolve(components []Component, cfg *Config) []Component {
	kind := ""
	if cfg != nil {
		kind = cfg.String(kindField)
	}

	return lo.Map(components, func(c Component, _ int) Component {
		c.Secrets = lo.Map(c.Secrets, func(s Secret, _ int) Secret {
			if path, found := s.PathByKind[kind]; found {
				s.Path = path
			}

			return s
		})

		return c
	})
}

// Encryption returns the component that encrypts the secrets of etcd at rest, and whether the
// cluster can use it. A field that holds a value already gives the answer, thus furyctl asks the
// user only about an empty one.
func Encryption(cfg *Config, schema *Schema) (Component, bool) {
	component, found := lo.Find(All(), func(c Component) bool { return c.Name == EncryptionComponent })
	if !found || cfg == nil {
		return component, false
	}

	// The read comes after this answer. A cluster with no such field holds no answer in it, and a
	// read that furyctl cannot make would stop the run over a field that the cluster cannot hold.
	if !holdsEncryption(cfg, schema) {
		return component, false
	}

	// A field that furyctl cannot read is not an empty field. An offer to write a new value over
	// such a field gives the cluster a second key, and the secrets that etcd already holds then
	// stay unreadable. Unreadable names the field, and the command stops.
	//
	// The read comes first, thus readable answers about the field of this cluster.
	value := cfg.String(etcdEncryptionConfiguration)

	return component, value == "" && cfg.readable(etcdEncryptionConfiguration)
}

// Unreadable returns each field that furyctl must read and cannot. Those are the fields that
// decide which components a cluster uses, and the field of each secret that furyctl is about to
// write.
//
// A field that holds a reference furyctl cannot follow says nothing about the cluster. The cluster
// can run with a secret that furyctl cannot see, thus a new value is a second secret and the
// cluster keeps the first one. With the encryption at rest of etcd, the secrets that etcd already
// holds stay unreadable after a new key.
//
// The read of each field happens here, and not where each caller reads it. A field that only one
// path of the command reads leaves that path with a guard and the other one without.
func Unreadable(components []Component, cfg *Config, schema *Schema, names []string) []string {
	if cfg == nil {
		return nil
	}

	// Select read the fields of every component, and Keep then dropped the components that this
	// cluster has no field for. A field of a component that this cluster cannot hold decides
	// nothing here, thus the answer comes from a new read of what is left.
	cfg.forget()
	cfg.String(kindField)

	// The field of the encryption at rest decides the question that the command asks, thus it
	// belongs here even when no component holds it yet: see Encryption.
	if holdsEncryption(cfg, schema) {
		cfg.String(etcdEncryptionConfiguration)
	}

	// The fields that decide come from every component that this cluster can hold, and not from the
	// components that Select chose. A component that Select left out because furyctl could not read
	// its field must name that field: the answer of the field is the reason the component is not
	// here. A component that this cluster has no field for must name nothing.
	//
	// A name on the command line says what to create, thus Select reads no field that decides and
	// the answer holds none of them either.
	//
	// The fields of a kind are not the fields of another one, thus resolve comes first: see resolve.
	// Without it the components of the Immutable kind look like components that no cluster can hold.
	// This walk keeps its own filter instead of Keep, because Keep reports each secret that it drops
	// and the command called it already.
	if len(names) == 0 {
		for _, component := range resolve(All(), cfg) {
			holdable := lo.SomeBy(component.Secrets, func(s Secret) bool { return schema.Has(s.Path) })

			if component.Needed != nil && holdable {
				component.Needed(cfg)
			}
		}
	}

	for _, component := range components {
		for _, secret := range component.Secrets {
			cfg.String(secret.Path)
		}
	}

	return cfg.Unresolved()
}

// holdsEncryption reports whether the cluster has the field for the encryption at rest. Without a
// schema the kinds that hold the field give the answer. This is the one answer that stays
// conservative: furyctl asks the user a question about the field, and a question about a field that
// the cluster cannot hold is worse than no question.
func holdsEncryption(cfg *Config, schema *Schema) bool {
	if schema != nil {
		return schema.Has(etcdEncryptionConfiguration)
	}

	return slices.Contains(
		[]string{distribution.OnPremisesKind, distribution.ImmutableKind},
		cfg.String(kindField),
	)
}

// Held returns the components with only the secrets that furyctl must create, and the field of each
// secret that the cluster holds somewhere else already. Such a field holds a value, and the file
// that furyctl would write is not there: the value of the field is thus not the value of that file.
// A new file would give the cluster a second secret, and the cluster would keep the first one.
//
// A file that is already there is the secret of its field, thus furyctl keeps that file and reports
// it: see writeNew.
func Held(components []Component, cfg *Config, folder string) ([]Component, []string) {
	if cfg == nil {
		return components, nil
	}

	kept := make([]Component, 0, len(components))
	held := []string{}

	for _, component := range components {
		component.Secrets = lo.Filter(component.Secrets, func(s Secret, _ int) bool {
			if cfg.String(s.Path) == "" {
				return true
			}

			if _, err := os.Stat(filepath.Join(folder, component.Name, s.Name)); err == nil {
				return true
			}

			held = append(held, s.Path)

			return false
		})

		if len(component.Secrets) == 0 {
			continue
		}

		kept = append(kept, component)
	}

	return kept, held
}

// Names returns the name of each component.
func Names() []string {
	return lo.Map(All(), func(c Component, _ int) string { return c.Name })
}

// Write creates the secrets of the components and writes each one to its own file. It makes one
// folder for each component inside the folder that you give. A file that is already there keeps its
// value: furyctl never writes a secret twice, thus a second run cannot lock a live cluster out.
func Write(components []Component, folder string) ([]Result, error) {
	results := []Result{}
	open := []string{}
	faults := []error{}

	// The folder that holds each component comes first. Without it every component fails for the
	// same reason, and the run reports that reason once for each of them. The warning about the
	// files that other accounts can read still happens: the folder can hold the secrets of a run
	// before this one.
	if err := os.MkdirAll(folder, folderPermissions); err != nil {
		warnOpenPaths(folder, open)

		return results, fmt.Errorf("error while creating the folder %s: %w", folder, err)
	}

	for _, component := range components {
		dir := filepath.Join(folder, component.Name)

		// A fault of one path never stops the run. Each secret that furyctl creates thus gets its
		// file and its reference, and the error at the end names every path that furyctl could not
		// write. A stop leaves the files of this run with no field that reads them, with no warning
		// about the private keys in them, and the run after this one stops at the same path.
		if err := os.MkdirAll(dir, folderPermissions); err != nil {
			faults = append(faults, fmt.Errorf("error while creating the folder %s: %w", dir, err))

			continue
		}

		for _, secret := range component.Secrets {
			file := filepath.Join(dir, secret.Name)

			created, err := writeNew(file, secret.New)
			if err != nil {
				faults = append(faults, err)

				continue
			}

			// A file that furyctl creates gets 0600. A file that is already there keeps the
			// permissions that it has: git gives 0644 to each file that it writes from the index,
			// thus a clone of a repository holds secrets that each account can read.
			if !created && isOpen(file) {
				open = append(open, file)
			}

			results = append(results, Result{
				Path:    secret.Path,
				File:    file,
				Created: created,
			})
		}
	}

	warnOpenPaths(folder, open)

	return results, errors.Join(faults...)
}

// isOpen reports whether the group or the others can read the path.
func isOpen(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().Perm()&otherPermissions != 0
}

// warnOpenPaths tells the user about the folder and the files that the other accounts of the machine
// can read. MkdirAll keeps the permissions of a folder that is already there, and furyctl never
// changes the permissions of a file that it did not write.
func warnOpenPaths(folder string, files []string) {
	var subject string

	switch {
	case isOpen(folder) && len(files) > 0:
		subject = fmt.Sprintf("the folder %s and %d of the files in it", folder, len(files))

	case isOpen(folder):
		subject = "the folder " + folder

	case len(files) > 0:
		subject = fmt.Sprintf("%d of the files in %s", len(files), folder)

	default:
		return
	}

	logrus.Warnf(
		"Other accounts on this machine can read %s. To close the permissions, run: chmod -R go-rwx %s",
		subject, folder,
	)
}

// writeNew writes a new value to the file. It reports false and keeps the value that the file
// holds when the file is already there. The O_EXCL flag creates the file only when it is not there,
// in one operation. Thus two runs at the same time cannot write the same file.
//
// The run that loses that race can find the file of the other run before it holds its value. The
// keepsASecret function then reports a file with no value in it. Its message thus asks the user to
// look at the file, and does not ask for a remove.
func writeNew(file string, generate func() (string, error)) (bool, error) {
	// The value comes first: a file that furyctl creates and then cannot write stays empty, and
	// each run after this one keeps that empty file.
	value, err := generate()
	if err != nil {
		return false, err
	}

	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePermissions)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, keepsASecret(file)
		}

		return false, fmt.Errorf("error while creating the file %s: %w", file, err)
	}

	// The newline at the end makes the file easy to read with `cat` and in an editor. The
	// `{file://}` notation of the configuration file drops that newline.
	if _, err := f.WriteString(value + "\n"); err != nil {
		return false, errors.Join(
			fmt.Errorf("error while writing the file %s: %w", file, err),
			f.Close(),
			os.Remove(file),
		)
	}

	// Close reports the errors that the write kept in the buffer, for example a full disk.
	if err := f.Close(); err != nil {
		return false, errors.Join(
			fmt.Errorf("error while closing the file %s: %w", file, err),
			os.Remove(file),
		)
	}

	return true, nil
}

// keepsASecret reports whether the path that is already there holds a secret. A folder answers the
// O_EXCL flag in the same way as a file, and a file with no value in it gives the cluster an empty
// secret, thus furyctl looks at both.
//
// Only furyctl reads a `{file://}` value: a file that it cannot read gives the cluster nothing.
func keepsASecret(file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("%w: furyctl cannot read the path %s: %w", ErrFileHoldsNoSecret, file, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a file", ErrFileHoldsNoSecret, file)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("%w: furyctl cannot read the file %s: %w", ErrFileHoldsNoSecret, file, err)
	}

	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("%w: the file %s holds no value. Another run of furyctl can hold that file "+
			"open at this moment, thus look at the file before you remove it. furyctl writes a new "+
			"value after a remove",
			ErrFileHoldsNoSecret, file)
	}

	return nil
}

// key returns 32 random bytes in base64, the value of `head -c32 /dev/urandom | base64`.
func key() (string, error) {
	buf := make([]byte, keyLength)

	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("error while reading random bytes: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

// password returns a random string of upper-case letters and digits, with at least 128 bits of
// randomness. Such a value needs no quotes and no escape in a YAML file, in a URL, and in the
// environment file of a container.
func password() (string, error) {
	return rand.Text(), nil
}

// passphrase returns a random string of 8 characters for Keepalived. The length of the value of
// [rand.Text] is not part of its contract, thus the length of the passphrase is the shorter of the
// two.
func passphrase() (string, error) {
	text := rand.Text()

	return text[:min(passphraseLength, len(text))], nil
}

// encryptionConfiguration returns an EncryptionConfiguration object for the API server, with a new
// key for the secretbox provider. The identity provider comes last: without it the API server
// cannot read the secrets that etcd holds with no encryption.
func encryptionConfiguration() (string, error) {
	value, err := key()
	if err != nil {
		return "", err
	}

	// The name of the key is not the same twice. A cluster that holds a key already keeps that key
	// in its configuration, and the API server reads the keys by name: two keys with one name make
	// the API server read the wrong one, and the secrets that etcd holds stay unreadable.
	return `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - secretbox:
          keys:
            - name: ` + keyName() + `
              secret: ` + value + `
      # The identity provider reads the secrets that etcd holds with no encryption. Keep it last.
      - identity: {}`, nil
}

// keyName returns a name for a key of the encryption at rest that no other key holds.
func keyName() string {
	return "furyctl-" + strings.ToLower(rand.Text()[:keyNameLength])
}

// signingKey returns a new P-256 (ES256) private key, in PEM, in base64. It is the value of
// `openssl ecparam -genkey -name prime256v1 -noout -out ec_private.pem` and
// `cat ec_private.pem | base64`.
func signingKey() (string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("error while creating the signing key: %w", err)
	}

	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", fmt.Errorf("error while encoding the signing key: %w", err)
	}

	block := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	return base64.StdEncoding.EncodeToString(block), nil
}
