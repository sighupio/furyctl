// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package phases

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/merge"
)

// kubeConfig returns a furyctl config content carrying spec.kubernetes.vpcId.
func kubeConfig(vpcID any) map[any]any {
	return map[any]any{
		"spec": map[any]any{
			"kubernetes": map[any]any{
				"vpcId": vpcID,
			},
		},
	}
}

// TestDistribution_extractVpcIDFromPrevPhases covers vpc_id resolution and its
// fallback to spec.kubernetes.vpcId when output.json has no usable vpc_id.
func TestDistribution_extractVpcIDFromPrevPhases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		// Parsed furyctl config that feeds the merger base.
		furyctlConf map[any]any
		// When non-empty, it is written to output.json in the infra outputs
		// dir; when empty, no output.json file exists.
		outputJSON string
		dryRun     bool

		want            string
		wantErrIs       error
		wantErrContains string
	}{
		{
			name:        "output.json has a valid vpc_id and takes precedence over the config",
			furyctlConf: kubeConfig("vpc-from-config"),
			outputJSON:  `{"vpc_id":{"sensitive":false,"value":"vpc-from-output"}}`,
			want:        "vpc-from-output",
		},
		{
			name:        "output.json vpc_id is not a string returns a casting error",
			furyctlConf: kubeConfig("vpc-from-config"),
			outputJSON:  `{"vpc_id":{"value":42}}`,
			wantErrIs:   ErrCastingVpcIDToStr,
		},
		{
			name:        "output.json without vpc_id falls back to the config vpcId",
			furyctlConf: kubeConfig("vpc-from-config"),
			outputJSON:  `{"nat_gateway_ids":{"value":["nat-1"]}}`,
			want:        "vpc-from-config",
		},
		{
			name:        "output.json with a null vpc_id falls back to the config vpcId",
			furyctlConf: kubeConfig("vpc-from-config"),
			outputJSON:  `{"vpc_id":null}`,
			want:        "vpc-from-config",
		},
		{
			name:        "unparseable output.json falls back to the config vpcId",
			furyctlConf: kubeConfig("vpc-from-config"),
			outputJSON:  `}{ not json`,
			want:        "vpc-from-config",
		},
		{
			name:        "no output.json falls back to the config vpcId",
			furyctlConf: kubeConfig("vpc-from-config"),
			want:        "vpc-from-config",
		},
		{
			name:        "no output.json and vpcId not set returns a casting error",
			furyctlConf: kubeConfig(nil),
			wantErrIs:   ErrCastingVpcIDToStr,
		},
		{
			name:        "no output.json and vpcId not set is tolerated on dry-run",
			furyctlConf: kubeConfig(nil),
			dryRun:      true,
		},
		{
			name:            "no output.json and missing spec.kubernetes returns a wrapped error",
			furyctlConf:     map[any]any{"spec": map[any]any{}},
			wantErrContains: "error getting kubernetes from furyctl config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outputsDir := t.TempDir()

			if tc.outputJSON != "" {
				if err := os.WriteFile(
					filepath.Join(outputsDir, "output.json"),
					[]byte(tc.outputJSON),
					0o600,
				); err != nil {
					t.Fatalf("error writing output.json: %v", err)
				}
			}

			d := &Distribution{
				DryRun:                             tc.dryRun,
				InfrastructureTerraformOutputsPath: outputsDir,
			}

			base := merge.NewDefaultModel(tc.furyctlConf, ".spec.kubernetes")
			got, err := d.extractVpcIDFromPrevPhases(merge.NewMerger(base, base))

			switch {
			case tc.wantErrIs != nil:
				require.ErrorIs(t, err, tc.wantErrIs)

			case tc.wantErrContains != "":
				require.ErrorContains(t, err, tc.wantErrContains)

			default:
				require.NoError(t, err)
			}

			assert.Equal(t, tc.want, got)
		})
	}
}
