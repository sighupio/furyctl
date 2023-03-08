// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legacy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var ErrFuryFileUnmarshal = errors.New("error unmarshaling furyfile")

const (
	defaultVendorFolderName = "vendor"
)

type VersionPattern map[string]string

type ProviderOptSpec struct {
	Name  string `mapstructure:"name"`
	Label string `mapstructure:"label"`
}

type Package struct {
	Name         string          `yaml:"name"`
	Version      string          `yaml:"version"`
	URL          string          `yaml:"url"`
	Dir          string          `yaml:"dir"`
	Kind         string          `yaml:"kind"`
	ProviderOpt  ProviderOptSpec `mapstructure:"provider"`
	ProviderKind ProviderKind    `mapstructure:"providerKind"`
	Registry     bool            `mapstructure:"registry"`
}

type RegistrySpec struct {
	BaseURI string `mapstructure:"url"`
	Label   string `mapstructure:"label"`
}

type ProviderPattern map[string]ProviderKind

type FuryFile struct {
	VendorFolderName string          `yaml:"vendorFolderName"`
	Versions         VersionPattern  `yaml:"versions"`
	Roles            []Package       `yaml:"roles"`
	Modules          []Package       `yaml:"modules"`
	Bases            []Package       `yaml:"bases"`
	Provider         ProviderPattern `mapstructure:"provider"`
}

func NewFuryFile(path string) (*FuryFile, error) {
	ff, err := yamlx.FromFileV3[FuryFile](path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFuryFileUnmarshal, err)
	}

	if ff.VendorFolderName == "" {
		ff.VendorFolderName = defaultVendorFolderName
	}

	return &ff, nil
}

func (f *FuryFile) BuildPackages(prefix string) ([]Package, error) {
	pkgs := make([]Package, 0)

	if prefix != "" {
		logrus.Infof("prefix is set to '%s', downloading only matching modules", prefix)
	}

	for _, v := range f.Roles {
		v.Kind = "roles"
		if strings.HasPrefix(v.Name, prefix) {
			logrus.Debugf("role '%s' matches prefix, adding it to the download list", v.Name)
			pkgs = append(pkgs, v)
		} else {
			logrus.Debugf("role '%s' does not match prefix, skipping it", v.Name)
		}
	}

	for _, v := range f.Modules {
		v.Kind = "modules"
		if strings.HasPrefix(v.Name, prefix) {
			logrus.Debugf("module '%s' matches prefix, adding it to the download list", v.Name)
			pkgs = append(pkgs, v)
		} else {
			logrus.Debugf("module '%s' does not match prefix, skipping it", v.Name)
		}
	}

	for _, v := range f.Bases {
		v.Kind = "katalog"
		if strings.HasPrefix(v.Name, prefix) {
			logrus.Debugf("katalog '%s' matches prefix, adding it to the download list", v.Name)
			pkgs = append(pkgs, v)
		} else {
			logrus.Debugf("katalog '%s' does not match prefix, skipping it", v.Name)
		}
	}

	for i := range pkgs {
		for k, v := range f.Versions {
			if strings.HasPrefix(pkgs[i].Name, k) {
				pkgs[i].Version = v

				break
			}
		}

		pkgs[i].ProviderKind = f.Provider[pkgs[i].Kind]
		pkgs[i].Dir = newDir(f.VendorFolderName, pkgs[i]).getConsumableDirectory()
	}

	return pkgs, nil
}
