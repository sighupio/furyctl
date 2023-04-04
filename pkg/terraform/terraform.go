// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/terraform-exec/tfexec"
	log "github.com/sirupsen/logrus"
)

type Options struct {
	Version string

	Backend            string
	BackendConfig      map[string]string
	ReconfigureBackend bool
	UpgradeDeps        bool

	WorkingDir string
	ConfigDir  string

	GitHubToken string

	LogDir string
	Debug  bool
}

func NewExecutor(opts Options) (tf *tfexec.Terraform, err error) {
	downloadPath := fmt.Sprintf("%v/bin", opts.WorkingDir)
	tfBinary, err := ensure(opts.Version, downloadPath)
	if err != nil {
		return nil, err
	}
	tf, err = tfexec.NewTerraform(opts.WorkingDir, tfBinary)
	if err != nil {
		return nil, err
	}
	err = configureLogger(tf, opts.WorkingDir, opts.LogDir, opts.Debug)
	if err != nil {
		return nil, err
	}
	err = createBackendFile(opts.WorkingDir, opts.Backend, opts.BackendConfig)
	if err != nil {
		return nil, err
	}
	// Gets all os environment
	netRcEnv := envMap(os.Environ())
	if opts.GitHubToken != "" {
		err = configureGitHubNetrcAccess(opts.WorkingDir, opts.GitHubToken, opts.ConfigDir)
		if err != nil {
			return nil, err
		}
		// Adds/Override NETRC to use our own netrc file
		netRcEnv["NETRC"] = fmt.Sprintf("%v/%v/.netrc", opts.WorkingDir, opts.ConfigDir)
	}
	// Set the env to the executor
	err = tf.SetEnv(netRcEnv)
	if err != nil {
		return nil, err
	}
	return tf, err
}

var forbidenTerraformEnvs = map[string]bool{
	"TF_LOG":                  true,
	"TF_INPUT":                true,
	"TF_IN_AUTOMATION":        true,
	"TF_LOG_PATH":             true,
	"TF_REATTACH_PROVIDERS":   true,
	"TF_APPEND_USER_AGENT":    true,
	"TF_WORKSPACE":            true,
	"TF_DISABLE_PLUGIN_TLS":   true,
	"TF_SKIP_PROVIDER_VERIFY": true,
}

const (
	varEnvVarPrefix = "TF_VAR_"
)

func envMap(environ []string) map[string]string {
	env := map[string]string{}
	for _, ev := range environ {
		parts := strings.SplitN(ev, "=", 2)
		if len(parts) == 0 {
			continue
		}
		k := parts[0]
		v := ""
		if len(parts) == 2 {
			v = parts[1]
		}
		if !strings.HasPrefix(k, varEnvVarPrefix) && !forbidenTerraformEnvs[k] {
			env[k] = v
		} else {
			log.Warnf("%v Environment variable discarted. Executor will not use it", k)
		}
	}
	return env
}

func configureLogger(tf *tfexec.Terraform, workingDir string, logDir string, debug bool) (err error) {
	logFile, err := os.Create(fmt.Sprintf("%v/%v/terraform.logs", workingDir, logDir))
	tf.SetLogger(log.StandardLogger())
	c := &tfwriter{
		logfile: logFile,
		debug:   debug,
	}
	if err != nil {
		log.Errorf("Can not init log file. %v", err)
		return err
	}
	tf.SetStdout(c)
	tf.SetStderr(c)
	return nil
}

// configureGitHubNetrcAccess creates the .netrc file with the credentials to access github private repos
func configureGitHubNetrcAccess(path string, token string, configDir string) (err error) {
	netrc := fmt.Sprintf(`machine github.com
login furyctl
password %v
`, token)
	return ioutil.WriteFile(fmt.Sprintf("%v/%v/.netrc", path, configDir), []byte(netrc), os.FileMode(0644))
}

// CreateBackendFile creates the backend.tf terraform file with the backend configuration chosen
func createBackendFile(path string, backend string, backendConfig map[string]string) (err error) {
	var backendFilebuffer bytes.Buffer
	backendFilebuffer.WriteString(fmt.Sprintf(`terraform {
  backend "%v" {
`, backend))
	for k, v := range backendConfig {
		backendFilebuffer.WriteString(fmt.Sprintf("    %v = \"%v\"\n", k, v))
	}
	backendFilebuffer.WriteString(`  }
}`)
	backendFileContent := backendFilebuffer.Bytes()
	return ioutil.WriteFile(fmt.Sprintf("%v/backend.tf", path), backendFileContent, os.FileMode(0644))
}

type tfwriter struct {
	logfile *os.File
	debug   bool
}

func (c *tfwriter) Write(data []byte) (n int, err error) {
	n, err = c.logfile.Write(data)
	if err != nil {
		return 0, err
	}
	if c.debug {
		fmt.Print(string(data))
	}
	return n, nil
}
