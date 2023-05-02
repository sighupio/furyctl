// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import "strings"

const MinDiffLineTokensNum = 3

type TfPlanParser struct {
	Plan string
}

type TfPlan struct {
	Destroy []string
	Add     []string
	Change  []string
}

func NewTfPlanParser(plan string) *TfPlanParser {
	return &TfPlanParser{
		Plan: plan,
	}
}

func (p *TfPlanParser) Parse() *TfPlan {
	pl := TfPlan{
		Destroy: []string{},
		Add:     []string{},
		Change:  []string{},
	}

	planLines := strings.Split(p.Plan, "\n")

	for i, line := range planLines {
		line = strings.TrimSpace(line)

		if !strings.HasPrefix(line, "#") || i+1 >= len(planLines) {
			continue
		}

		diffLineTokens := strings.Split(strings.TrimSpace(planLines[i+1]), " ")

		if len(diffLineTokens) < MinDiffLineTokensNum {
			continue
		}

		resourceName := strings.Trim(diffLineTokens[2], "\"")

		switch diffLineTokens[0] {
		case "+":
			pl.Add = append(pl.Add, resourceName)

		case "-":
			pl.Destroy = append(pl.Destroy, resourceName)

		case "~":
			pl.Change = append(pl.Change, resourceName)

		case "-/+":
			pl.Destroy = append(pl.Destroy, resourceName)
			pl.Add = append(pl.Add, resourceName)
		}
	}

	return &pl
}
