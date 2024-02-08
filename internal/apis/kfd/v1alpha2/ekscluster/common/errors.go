// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import "errors"

const SErrWrapWithStr = "%w: %s"

var (
	ErrVpcIDNotFound     = errors.New("vpc_id not found in infra output")
	ErrVpcIDFromOut      = errors.New("cannot read vpc_id from infrastructure's output.json")
	ErrWritingTfVars     = errors.New("error writing terraform variables file")
	ErrCastingVpcIDToStr = errors.New("error casting vpc_id output to string")
	ErrVpcCIDRNotFound   = errors.New("vpc_cidr_block not found in infra output")
	ErrPvtSubnetFromOut  = errors.New("cannot read private_subnets from infrastructure's output.json")
	ErrVpcCIDRFromOut    = errors.New("cannot read vpc_cidr_block from infrastructure's output.json")
	ErrPvtSubnetNotFound = errors.New("private_subnets not found in infrastructure phase's output")
)
