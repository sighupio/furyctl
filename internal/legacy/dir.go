// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legacy

import "fmt"

type DirSpec struct {
	VendorFolder string
	Kind         string
	Name         string
	Registry     bool
	Provider     ProviderOptSpec
}

func newDir(vendorFolder string, pkg Package) *DirSpec {
	return &DirSpec{
		VendorFolder: vendorFolder,
		Kind:         pkg.Kind,
		Name:         pkg.Name,
		Registry:     pkg.Registry,
		Provider:     pkg.ProviderOpt,
	}
}

func (d *DirSpec) getConsumableDirectory() string {
	if d.Registry {
		return fmt.Sprintf("%s/%s/%s/%s/%s", d.VendorFolder, d.Kind, d.Provider.Label, d.Provider.Name, d.Name)
	}

	return fmt.Sprintf("%s/%s/%s", d.VendorFolder, d.Kind, d.Name)
}
