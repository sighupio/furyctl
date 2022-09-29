// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cobrax_test

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/cobrax"
)

func Test_Flag_Bool(t *testing.T) {
	testCases := []struct {
		desc string
		cmd  *cobra.Command
		args []string
		name string
		want bool
	}{
		{
			desc: "empty cobra command",
			cmd:  nil,
			name: "",
			want: false,
		},
		{
			desc: "non-existing flag",
			cmd:  &cobra.Command{},
			name: "wrong",
			want: false,
		},
		{
			desc: "existing flag, value true",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().BoolP("debug", "D", false, "Enables furyctl debug output")

				return cmd
			}(),
			args: []string{"--debug=true"},
			name: "debug",
			want: true,
		},
		{
			desc: "existing flag, value false",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().BoolP("debug", "D", false, "Enables furyctl debug output")

				return cmd
			}(),
			args: []string{"--debug=false"},
			name: "debug",
			want: false,
		},
		{
			desc: "existing flag, wrong value",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().BoolP("debug", "D", false, "Enables furyctl debug output")

				return cmd
			}(),
			args: []string{"--debug=wrong"},
			name: "debug",
			want: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if tC.cmd != nil {
				tC.cmd.SetArgs(tC.args)
				tC.cmd.Execute()
			}

			if got := cobrax.Flag[bool](tC.cmd, tC.name); got != tC.want {
				t.Errorf("expected %t, got %t", tC.want, got)
			}
		})
	}
}

func Test_Flag_Int(t *testing.T) {
	testCases := []struct {
		desc string
		cmd  *cobra.Command
		args []string
		name string
		want int
	}{
		{
			desc: "empty cobra command",
			cmd:  nil,
			name: "",
			want: 0,
		},
		{
			desc: "non-existing flag",
			cmd:  &cobra.Command{},
			name: "wrong",
			want: 0,
		},
		{
			desc: "existing flag, value 123",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().IntP("count", "C", 123, "Number of times to run")

				return cmd
			}(),
			args: []string{"--count=123"},
			name: "count",
			want: 123,
		},
		{
			desc: "existing flag, value 0",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().IntP("count", "C", 0, "Number of times to run")

				return cmd
			}(),
			args: []string{"--count=0"},
			name: "count",
			want: 0,
		},
		{
			desc: "existing flag, wrong value",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().IntP("count", "C", 0, "Number of times to run")

				return cmd
			}(),
			args: []string{"--count=wrong"},
			name: "count",
			want: 0,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if tC.cmd != nil {
				tC.cmd.SetArgs(tC.args)
				tC.cmd.Execute()
			}

			if got := cobrax.Flag[int](tC.cmd, tC.name); got != tC.want {
				t.Errorf("expected %d, got %d", tC.want, got)
			}
		})
	}
}

func Test_Flag_String(t *testing.T) {
	testCases := []struct {
		desc string
		cmd  *cobra.Command
		args []string
		name string
		want string
	}{
		{
			desc: "empty cobra command",
			cmd:  nil,
			name: "",
			want: "",
		},
		{
			desc: "non-existing flag",
			cmd:  &cobra.Command{},
			name: "wrong",
			want: "",
		},
		{
			desc: "existing flag, value bar",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().StringP("foo", "F", "", "Example")

				return cmd
			}(),
			args: []string{"--foo=bar"},
			name: "foo",
			want: "bar",
		},
		{
			desc: "existing flag, empty value",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}

				cmd.Flags().StringP("foo", "F", "", "Example")

				return cmd
			}(),
			args: []string{"--foo="},
			name: "foo",
			want: "",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if tC.cmd != nil {
				tC.cmd.SetArgs(tC.args)
				tC.cmd.Execute()
			}

			if got := cobrax.Flag[string](tC.cmd, tC.name); got != tC.want {
				t.Errorf("expected %s, got %s", tC.want, got)
			}
		})
	}
}
