package app_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/execx"
)

func TestValidateDependencies(t *testing.T) {
	testCases := []struct {
		desc         string
		executor     execx.Executor
		envs         map[string]string
		kfdConf      distribution.Manifest
		wantErrCount int
		wantErrVal   any
		wantErrType  error
	}{
		{
			desc:         "missing tools and envs",
			executor:     execx.NewStdExecutor(),
			kfdConf:      correctKFDConf,
			wantErrCount: 8,
			wantErrVal:   &fs.PathError{},
			wantErrType:  app.ErrMissingEnvVar,
		},
		{
			desc:     "has all tools and envs",
			executor: execx.NewFakeExecutor(),
			kfdConf:  correctKFDConf,
			envs: map[string]string{
				"AWS_ACCESS_KEY_ID":     "test",
				"AWS_SECRET_ACCESS_KEY": "test",
				"AWS_DEFAULT_REGION":    "test",
			},
			wantErrCount: 0,
		},
		{
			desc:     "has wrong tools",
			executor: execx.NewFakeExecutor(),
			kfdConf:  wrongKFDConf,
			envs: map[string]string{
				"AWS_ACCESS_KEY_ID":     "test",
				"AWS_SECRET_ACCESS_KEY": "test",
				"AWS_DEFAULT_REGION":    "test",
			},
			wantErrCount: 5,
			wantErrType:  app.ErrWrongToolVersion,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			tmpDir, configFilePath := setupDistroFolder(t, correctFuryctlDefaults, tC.kfdConf)
			defer rmDirTemp(t, tmpDir)

			for k, v := range tC.envs {
				t.Setenv(k, v)
			}

			vd := app.NewValidateDependencies(tC.executor)

			res, err := vd.Execute(app.ValidateDependenciesRequest{
				BinPath:           filepath.Join(tmpDir, "bin"),
				FuryctlBinVersion: "unknown",
				DistroLocation:    tmpDir,
				FuryctlConfPath:   configFilePath,
				Debug:             true,
			})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tC.wantErrCount != len(res.Errors) {
				t.Errorf("Expected %d validation errors, got %d", tC.wantErrCount, len(res.Errors))
				for _, err := range res.Errors {
					t.Log(err)
				}
			}

			for _, err := range res.Errors {
				notErrAs := tC.wantErrVal != nil && !errors.As(err, &tC.wantErrVal)
				notErrIs := tC.wantErrType != nil && !errors.Is(err, tC.wantErrType)

				if notErrAs && notErrIs {
					t.Fatalf("got error = %v, want = %v", err, tC.wantErrVal)
				}
			}
		})
	}
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, _ := args[3], args[4:]

	switch cmd {
	case "ansible":
		fmt.Fprintf(os.Stdout, "ansible [core 2.11.2]\n  "+
			"config file = None\n  "+
			"configured module search path = ['', '']\n"+
			"ansible python module location = ./ansible\n"+
			"ansible collection location = ./ansible/collections\n"+
			"executable location = ./bin/ansible\n  "+
			"python version = 3.9.14\n"+
			"jinja version = 3.1.2\n"+
			"libyaml = True\n")
	case "terraform":
		fmt.Fprintf(os.Stdout, "Terraform v0.15.4\non darwin_amd64")
	case "kubectl":
		fmt.Fprintf(os.Stdout, "Client Version: version.Info{Major:\"1\", "+
			"Minor:\"21\", GitVersion:\"v1.21.1\", GitCommit:\"xxxxx\", "+
			"GitTreeState:\"clean\", BuildDate:\"2021-05-12T14:00:00Z\", "+
			"GoVersion:\"go1.16.4\", Compiler:\"gc\", Platform:\"darwin/amd64\"}\n")
	case "kustomize":
		fmt.Fprintf(os.Stdout, "Version: {kustomize/v3.9.4 GitCommit:xxxxxxx"+
			"BuildDate:2021-05-12T14:00:00Z GoOs:darwin GoArch:amd64}")
	case "furyagent":
		fmt.Fprintf(os.Stdout, "furyagent version 0.0.1")
	default:
		fmt.Fprintf(os.Stdout, "command not found")
	}

	os.Exit(0)
}
