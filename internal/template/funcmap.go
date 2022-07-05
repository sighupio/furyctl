// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

type FuncMap struct {
	FuncMap template.FuncMap
}

func NewFuncMap() FuncMap {
	return FuncMap{FuncMap: sprig.TxtFuncMap()}
}

func (f *FuncMap) Add(name string, fn interface{}) {
	f.FuncMap[name] = fn
}

func (f *FuncMap) Delete(name string) {
	delete(f.FuncMap, name)
}
