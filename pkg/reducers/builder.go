// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reducers

import (
	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

func Build(
	statusDiffs r3diff.Changelog,
	rulesExtractor rules.Extractor,
	phase string,
) Reducers {
	reducersRules := rulesExtractor.GetReducers(phase)

	filteredReducers := rulesExtractor.ReducerRulesByDiffs(reducersRules, statusDiffs)

	rdcs := make(Reducers, len(filteredReducers))

	if len(filteredReducers) > 0 {
		for _, reducer := range filteredReducers {
			if reducer.Reducers != nil {
				if reducer.Description != nil {
					logrus.Infof("%s", *reducer.Description)
				}

				for _, red := range *reducer.Reducers {
					rdcs = append(rdcs, NewBaseReducer(
						red.Key,
						red.From,
						red.To,
						red.Lifecycle,
						reducer.Path,
					),
					)
				}
			}
		}
	}

	return rdcs
}
