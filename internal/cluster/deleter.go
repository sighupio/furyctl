// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cluster

import (
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
)

const (
	DeleterPropertyPhase = "phase"
)

var delFactories = make(map[string]map[string]DeleterFactory) //nolint:gochecknoglobals, lll // This patterns requires factories
//  as global to work with init function.

type DeleterFactory func(props []DeleterProperty) (Deleter, error)

type DeleterProperty struct {
	Name  string
	Value any
}

type Deleter interface {
	SetProperties(props []DeleterProperty)
	SetProperty(name string, value any)
	Delete(dryRun bool) error
}

func NewDeleter(
	minimalConf config.Furyctl,
	phase string,
) (Deleter, error) {
	lcAPIVersion := strings.ToLower(minimalConf.APIVersion)
	lcResourceType := strings.ToLower(minimalConf.Kind)

	if factoryFn, ok := delFactories[lcAPIVersion][lcResourceType]; ok {
		return factoryFn([]DeleterProperty{
			{
				Name:  DeleterPropertyPhase,
				Value: phase,
			},
		})
	}

	return nil, fmt.Errorf("%w -  type '%s' api version '%s'", errResourceNotSupported, lcResourceType, lcAPIVersion)
}

func RegisterDeleterFactory(apiVersion, kind string, factory DeleterFactory) {
	lcAPIVersion := strings.ToLower(apiVersion)
	lcKind := strings.ToLower(kind)

	if _, ok := delFactories[lcAPIVersion]; !ok {
		delFactories[lcAPIVersion] = make(map[string]DeleterFactory)
	}

	delFactories[lcAPIVersion][lcKind] = factory
}

func NewDeleterFactory[T Deleter](dd T) DeleterFactory {
	return func(props []DeleterProperty) (Deleter, error) {
		dd.SetProperties(props)

		return dd, nil
	}
}
