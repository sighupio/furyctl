// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	distroconf "github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/secrets"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// ErrUnreadableField reports a field of the configuration file that holds a value furyctl cannot
// read. Such fields decide which secrets a cluster needs, thus the command stops.
var ErrUnreadableField = errors.New("cannot read the value of a field of the configuration file")

// The name that DoDownload gives to the folder of a distribution that it writes: see readSchema.
const tempFolderPrefix = "furyctl-"

func NewSecretsCmd() *cobra.Command {
	var cmdEvent analytics.Event

	secretsCmd := &cobra.Command{
		Args:      cobra.ArbitraryArgs,
		ValidArgs: secrets.Names(),
		Use:       "secrets [COMPONENT...]",
		Short:     "Creates the random secrets that the configuration file of a cluster needs.",
		Long: `Creates the random secrets that the configuration file of a cluster needs. For example, ` +
			`Pomerium needs four secrets when the authentication provider type is "sso".

furyctl writes one file for each secret, in a folder with the name of the component. The ` +
			`configuration file reads the value of each file with the "{file://<path>}" notation. The command ` +
			`prints the fields to add to the configuration file.

With no argument, furyctl creates the secrets of the components that the configuration file uses. ` +
			`When there is no configuration file, furyctl creates the secrets of every component. You can ` +
			`name the components to create only some of them, for example ` +
			`"furyctl create secrets pomerium keepalived".

The components are: ` + strings.Join(secrets.Names(), ", ") + `.

On the OnPremises and Immutable kinds furyctl also asks whether the cluster encrypts the secrets of ` +
			`etcd at rest. It writes that configuration only after an answer of "yes", or when you name the ` +
			`component. It asks nothing with the --no-tty flag.

A file that is already there keeps its value: furyctl never writes a secret twice. Thus you can run ` +
			`the command again after you add a module to the configuration file.

furyctl creates only the secrets that your cluster can hold. The kind of the cluster and the ` +
			`version of the distribution decide them, thus an older version gets fewer secrets.`,
		Example: `  furyctl create secrets                            Create the secrets that furyctl.yaml needs
  furyctl create secrets pomerium                   Create only the secrets of Pomerium
  furyctl create secrets --path ./mysecrets         Write the files in another folder
  furyctl create secrets --update-config            Also write the references in furyctl.yaml
  furyctl create secrets --config mycluster.yaml    Read another configuration file
`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// The command reads no flag from the `flags` field of the configuration file: give
			// --distro-location on the command line. Only `create pki` reads the `create` section
			// of that field, and there `path` means the folder of the PKI and not the folder of the
			// secrets.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, args []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			fail := func(err error) error {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			// Each message of the command names a path, and `create pki` names an absolute one. An
			// absolute path leaves no doubt about the folder that holds the secrets, above all with
			// the --workdir flag. The `{file://}` references stay relative to the configuration
			// file: Reference makes them so.
			folder, err := filepath.Abs(viper.GetString("path"))
			if err != nil {
				return fail(fmt.Errorf("error while getting the absolute path of the secrets folder: %w", err))
			}

			configPath, err := filepath.Abs(viper.GetString("config"))
			if err != nil {
				return fail(fmt.Errorf("error while getting the absolute path of the configuration file: %w", err))
			}

			updateConfig := viper.GetBool("update-config")

			// The command needs no configuration file. Without one, furyctl creates the secrets of
			// every component. Only the --update-config flag needs the file.
			cfg, err := secrets.LoadConfig(configPath)

			switch {
			case err == nil:

			case !errors.Is(err, os.ErrNotExist):
				return fail(fmt.Errorf("error while reading the configuration file: %w", err))

			case updateConfig:
				return fail(fmt.Errorf("cannot write the references with the --update-config flag: %w", err))

			// The names on the command line already say what to create, thus the absent file is no
			// surprise. Without them, furyctl tells the user why it creates so many secrets.
			case len(args) == 0:
				logrus.Warnf(
					"The configuration file %s is not there, thus furyctl creates the secrets of every "+
						"component. To create fewer secrets, name the components you need. If your "+
						"configuration file has another path, give it with the --config flag.",
					configPath,
				)

			default:
			}

			// The kind of the cluster and the version of the distribution decide which fields the
			// cluster has, and the public schema of that version holds them. Without it furyctl
			// would create secrets for fields that the version has no place for.
			var schema *secrets.Schema

			if cfg != nil {
				if schema, err = readSchema(configPath); err != nil {
					logrus.Warnf(
						"furyctl cannot read the schema of the cluster of %s: %v. It creates the secrets "+
							"of every component that the configuration file uses. Give a local copy of the "+
							"distribution with the --distro-location flag, or make sure that furyctl can "+
							"reach the repository of the distribution.",
						configPath, err,
					)
				}
			}

			components, err := secrets.Select(args, cfg)
			if err != nil {
				return fail(err)
			}

			components, dropped := secrets.Keep(components, schema)

			// A name on the command line asks for those secrets, and the cluster holds no field for
			// some of them. The run creates the secrets of the other names, and this error names the
			// rest at the end. A stop here leaves the secrets that the cluster does hold uncreated.
			var noFieldErr error

			if len(args) > 0 && len(dropped) > 0 {
				noFieldErr = fmt.Errorf("%w: %s. The kind of the cluster, and the version of the "+
					"distribution that %s names, decide which fields the cluster has",
					secrets.ErrNoField, strings.Join(dropped, ", "), configPath)
			}

			// The names on the command line say what to create. Without them, only the user can
			// answer whether the cluster encrypts the secrets of etcd at rest. Encryption reads the
			// last field that furyctl decides with, thus the check below covers every one of them.
			encryption := secrets.Component{}
			encrypts := false

			if len(args) == 0 {
				encryption, encrypts = secrets.Encryption(cfg, schema)
			}

			// A field whose value furyctl cannot read says nothing about the cluster. The command
			// stops here instead of a guess: a guess creates the secrets of the wrong components,
			// and it reads a field that holds a reference as a field that holds nothing.
			if unreadable := secrets.Unreadable(components, cfg, schema, args); len(unreadable) > 0 {
				return fail(fmt.Errorf("%w: %s. Give each field a value that furyctl can read, or take "+
					"it out of %s",
					ErrUnreadableField, strings.Join(unreadable, ", "), configPath))
			}

			if encrypts && askForEncryption() {
				components = append(components, encryption)
			}

			// The cluster holds a secret that its field names and that no file of furyctl holds. A new
			// file would be a second secret, and the cluster would keep the first one.
			components, held := secrets.Held(components, cfg, folder)

			if len(held) > 0 {
				logrus.Infof(
					"These fields hold a value already, thus furyctl wrote no secret for them: %s. To "+
						"get a new value, take the field out of %s and run the command again.",
					strings.Join(held, ", "), configPath,
				)

				warnHeldEncryption(held)
			}

			if len(components) == 0 {
				if noFieldErr != nil {
					return fail(noFieldErr)
				}

				if len(held) == 0 {
					logrus.Infof(
						"The configuration file %s needs no secret. Name a component to create its "+
							"secrets anyway: %s",
						configPath,
						strings.Join(secrets.Names(), ", "),
					)
				}

				cmdEvent.AddSuccessMessage("no secret to create")
				tracker.Track(cmdEvent)

				return nil
			}

			// Write goes on after a path that it cannot write, thus furyctl reports the secrets that
			// it created and writes the references to them. Without that, the files of this run stay
			// on the disk with no field that reads them.
			results, writeErr := secrets.Write(components, folder)

			if len(results) > 0 {
				report(results, folder)

				if updateConfig {
					err = updateConfigFile(results, configPath)

					// The fields went into the file without the schema of the cluster, thus furyctl
					// does not know whether this version of the distribution holds each of them.
					if err == nil && schema == nil {
						logrus.Warnf(
							"Run `furyctl validate config` to make sure that this version of the "+
								"distribution holds each field that furyctl wrote in %s.",
							configPath,
						)
					}
				} else {
					err = printSnippet(results, configPath)
				}

				warnEncryption(results, folder)
				warnSecretFiles(folder)
			}

			if err := errors.Join(noFieldErr, writeErr, err); err != nil {
				return fail(err)
			}

			cmdEvent.AddSuccessMessage("secrets successfully created at " + folder)
			tracker.Track(cmdEvent)

			return nil
		},
	}

	secretsCmd.Flags().StringP(
		"path",
		"p",
		"./secrets",
		"path where to save the created secrets. furyctl creates one subfolder for each component",
	)

	secretsCmd.Flags().Bool(
		"update-config",
		false,
		"write the reference to each file in the configuration file. furyctl edits only the lines of "+
			"those fields, thus the comments and the format of the rest of the file do not change. A "+
			"field that already holds another value does not change",
	)

	secretsCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	secretsCmd.Flags().String(
		"distro-location",
		"",
		"Location where to download the schemas of the distribution from. furyctl reads the schema "+
			"of the cluster to know which fields the cluster has. It can either be a local path "+
			"(eg: /path/to/distribution) or a remote URL (eg: "+
			"git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). Any format supported "+
			"by hashicorp/go-getter can be used",
	)

	return secretsCmd
}

// readSchema reads which fields the cluster of a configuration file has. It downloads the
// distribution of the version that the file names, and reads it from the local cache when it is
// there already.
//
// It calls DoDownload and not Download, because a failure of Download clears the whole cache of the
// user: the fields are a check that furyctl can go without, thus it never pays that price.
func readSchema(configPath string) (*secrets.Schema, error) {
	minimalConf, err := yamlx.FromFileV3[distroconf.Furyctl](configPath)
	if err != nil {
		return nil, fmt.Errorf("error while reading the configuration file: %w", err)
	}

	gitProtocol, err := git.ParseProtocol(viper.GetString("git-protocol"))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	distroLocation := viper.GetString("distro-location")
	client := netx.NewGoGetterClient()

	distrodl := dist.NewDownloader(client, gitProtocol, "")
	if distroLocation == "" {
		distrodl = dist.NewCachingDownloader(client, viper.GetString("outdir"), gitProtocol, "")
	}

	logrus.Info("Reading the schema of the cluster...")

	res, err := distrodl.DoDownload(distroLocation, minimalConf)
	if err != nil {
		return nil, fmt.Errorf("error while downloading the distribution: %w", err)
	}

	// DoDownload writes the distribution to a folder of its own and gives no way to remove it. The
	// schema is in memory after LoadSchema, thus the folder goes away with the run that made it.
	// The name of the folder is not part of what DoDownload gives back, thus furyctl removes only a
	// folder with the name that DoDownload uses: a removal of the wrong path takes the work of the
	// user with it.
	defer func() {
		folder := filepath.Dir(res.RepoPath)

		if strings.HasPrefix(filepath.Base(folder), tempFolderPrefix) {
			if err := osx.CleanupTempDir(folder); err != nil {
				logrus.Debugf("furyctl cannot remove the folder %s of the distribution: %v", folder, err)
			}
		}
	}()

	schema, err := secrets.LoadSchema(res.RepoPath, minimalConf)
	if err != nil {
		return nil, fmt.Errorf("error while reading the schema of the cluster: %w", err)
	}

	return schema, nil
}

// askForEncryption asks whether the cluster encrypts the secrets of etcd at rest. The configuration
// file says nothing about it: the presence of the field is the switch.
func askForEncryption() bool {
	if viper.GetBool("no-tty") {
		logrus.Infof(
			"This cluster can encrypt the secrets of etcd at rest. To create the configuration for "+
				"it, run `furyctl create secrets %s`.",
			secrets.EncryptionComponent,
		)

		return false
	}

	fmt.Printf(
		"\nThe API server can encrypt the secrets of etcd at rest. furyctl writes the " +
			"EncryptionConfiguration object for it, with a new key.\n" +
			"Do you want that object? Only 'yes' writes it.\n",
	)

	answer, err := iox.NewPrompter(bufio.NewReader(os.Stdin)).Ask("yes")
	if err != nil {
		logrus.Warnf(
			"furyctl cannot read the answer, thus it creates no configuration for the encryption "+
				"at rest: %v. In a script, use the --no-tty flag, or name the %s component.",
			err, secrets.EncryptionComponent,
		)

		return false
	}

	return answer
}

// warnEncryption tells the user what the new EncryptionConfiguration needs to work.
func warnEncryption(results []secrets.Result, folder string) {
	dir := filepath.Join(folder, secrets.EncryptionComponent)

	// Only a configuration that furyctl wrote in this run needs the kubernetes phase. A file that
	// was already there holds the configuration that the cluster runs with, and a file that furyctl
	// could not write holds nothing: both make this warning ask for an apply for no reason.
	configuration, found := lo.Find(results, func(r secrets.Result) bool {
		return r.Created && filepath.Dir(r.File) == dir
	})
	if !found {
		return
	}

	logrus.Warnf(
		"The API server reads the encryption at rest configuration when it starts. To give it the "+
			"new configuration, apply the kubernetes phase: `furyctl apply --phase kubernetes`. The "+
			"secrets that etcd holds stay in clear text until something writes them again. Keep the "+
			"file %s safe: without it, etcd holds secrets that nothing can read.",
		configuration.File,
	)
}

// warnHeldEncryption tells what a new key of the encryption at rest costs. Each other secret comes
// back with a new value: a service reads that value again and goes on. The keys of etcd do not. The
// API server reads a secret of etcd with the key that wrote it, thus a key that goes away takes
// those secrets with it.
func warnHeldEncryption(held []string) {
	if !lo.Contains(held, secrets.EncryptionField) {
		return
	}

	logrus.Warnf(
		"The %s field holds the keys that the API server reads the secrets of etcd with. Do not take "+
			"that field out of the configuration file to get a new key: the secrets that etcd holds "+
			"stay unreadable without the key that wrote them. To change the key, put a new key before "+
			"the key that is there, apply the kubernetes phase, and write each secret again. Keep the "+
			"old key in the field until that is done.",
		secrets.EncryptionField,
	)
}

// report tells which files furyctl created, and which ones were already there.
func report(results []secrets.Result, folder string) {
	created, kept := lo.FilterReject(results, func(r secrets.Result, _ int) bool { return r.Created })

	if len(created) > 0 {
		logrus.Infof("Created the secrets in %s: %s", folder, secretNames(created))
	}

	if len(kept) > 0 {
		logrus.Infof(
			"These files are already there, thus furyctl kept their value: %s",
			secretNames(kept),
		)
	}
}

// secretNames returns the name of the component and of the file of each secret, on one line.
func secretNames(results []secrets.Result) string {
	return strings.Join(lo.Map(results, func(r secrets.Result, _ int) string {
		return filepath.Join(filepath.Base(filepath.Dir(r.File)), filepath.Base(r.File))
	}), ", ")
}

// updateConfigFile writes the reference to the file of each secret in the configuration file.
func updateConfigFile(results []secrets.Result, configPath string) error {
	written, kept, err := secrets.UpdateConfig(configPath, results)
	if err != nil {
		return fmt.Errorf("error while writing the configuration file: %w", err)
	}

	if written > 0 {
		logrus.Infof("Wrote the fields of the secrets in the configuration file %s", configPath)
	} else {
		logrus.Infof("The configuration file %s reads each file already, thus furyctl changed nothing", configPath)
	}

	if len(kept) > 0 {
		logrus.Warnf(
			"These fields hold another value, thus furyctl did not change them: %s. "+
				"Write the reference to the file of the secret in them, or delete their value and "+
				"run the command again.",
			strings.Join(kept, ", "),
		)
	}

	return nil
}

// printSnippet prints the fields to add to the configuration file.
func printSnippet(results []secrets.Result, configPath string) error {
	snippet, err := secrets.Snippet(results, filepath.Dir(configPath))
	if err != nil {
		return fmt.Errorf("error while writing the fields of the secrets: %w", err)
	}

	fmt.Printf(
		"\nAdd these fields to %s, or run the command again with the --update-config flag:\n\n%s\n",
		configPath,
		snippet,
	)

	return nil
}
