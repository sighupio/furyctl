// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/Al-Pragliola/go-version"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/airgap"
	distroconf "github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/semver"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	ErrInvalidUpgradeFolder    = errors.New("upgrade folder name is not <from>-<to>")
	ErrUpgradePathsConfigField = errors.New("field is empty in the configuration file")
)

func NewUpgradePathsCmd() *cobra.Command {
	upgradePathsCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "upgrade-paths",
		Short: "Get available upgrade paths for the kind and version defined in the configuration file or a custom one.",
		Long:  `Get available upgrade paths for the kind and version defined in the configuration file or a custom one. If the "--from" or "--kind" parameters are specified, the command will give the upgrade path for those instead.`,
		Example: `  furyctl get upgrade-paths                                 shows the available upgrade paths for the kind and distribution version defined in the configuration file (furyctl.yaml by default)
  furyctl get upgrade-paths --from vX.Y.Z                   shows the available upgrade paths for the kind defined in the configuration file but for the version X.Y.Z instead.
  furyctl get upgrade-paths --kind OnPremises               shows the available upgrade paths for the version defined in the configuration file but for the OnPremises kind, even if the cluster is an EKSCluster, for example.
  furyctl get upgrade-paths --kind OnPremises --from X.Y.X  shows the available upgrade paths for the version X.Y.Z of the OnPremises kind, without reading the configuration file.
 `,
		PreRun: func(cmd *cobra.Command, _ []string) {
			// Bind the flags first: a flag on the command line has precedence over the configuration file.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			if err := flags.LoadAndMergeCommandFlags("get"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			// Get flags.
			furyctlPath := viper.GetString("config")
			fromVersion := viper.GetString("from")
			kind := viper.GetString("kind")

			// Check that the version passed by the user is semVer valid.
			if fromVersion != "" {
				if _, err := semver.NewVersion(fromVersion); err != nil {
					return fmt.Errorf("'%s' is not a valid version string: %w", fromVersion, err)
				}
			}

			// The upgrade paths are embedded in furyctl: only kind and version are needed from the configuration
			// file, not the distribution, its schemas or the tools.
			if kind == "" || fromVersion == "" {
				confKind, confVersion, err := ReadUpgradePathsConfig(furyctlPath)
				if err != nil {
					return err
				}

				if kind == "" {
					kind = confKind
					logrus.Debugf("got kind %s from the configuration file.", kind)
				}

				if fromVersion == "" {
					fromVersion = confVersion
					logrus.Debugf("got version %s from the configuration file.", fromVersion)
				}
			}

			// We don't need the "v" prefix in the version. Drop it if the user passes it.
			fromVersion, _ = strings.CutPrefix(fromVersion, "v")

			// Validate the kind. We don't need the normalised kind because we are checking against the folder names.
			if _, err := distribution.ValidateConfigKind(kind); err != nil {
				return fmt.Errorf("error while validating kind: %w", err)
			}

			globPattern := fmt.Sprintf("%s/%s/%s-*", "upgrades", strings.ToLower(kind), fromVersion)
			availablePaths, err := fs.Glob(configs.Tpl, globPattern)
			logrus.Debug("found folders: ", availablePaths)
			if err != nil {
				return fmt.Errorf("error while getting the upgrade paths for version %s: %w", fromVersion, err)
			}

			if len(availablePaths) > 0 {
				var targetVersions []string
				for _, match := range availablePaths {
					f, err := fs.Stat(configs.Tpl, match)
					if err != nil {
						return fmt.Errorf("error while checking filesystem: %w", err)
					}
					if f.IsDir() {
						fromToVersions := strings.Split(filepath.Base(match), "-")
						toVersion := fromToVersions[len(fromToVersions)-1]
						targetVersions = append(targetVersions, toVersion)
					}
				}
				logrus.Infof("Available upgrade paths for version %s of kind %s are: %s", fromVersion, kind, strings.Join(targetVersions, ", "))
			} else {
				logrus.Infof("There are no upgrade paths available for version %s of kind %s", fromVersion, kind)
			}

			return printUpgradePaths(kind, fromVersion)
		},
	}

	upgradePathsCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)

	upgradePathsCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	upgradePathsCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	upgradePathsCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the binaries",
	)

	airgap.RegisterFlags(upgradePathsCmd)

	upgradePathsCmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	upgradePathsCmd.Flags().String(
		"from",
		"",
		"Show upgrade paths for the version specified (eg. 1.29.2) instead of the distribution version in the configuration file",
	)

	upgradePathsCmd.Flags().StringP(
		"kind",
		"k",
		"",
		"Show upgrade paths for the kind of cluster specified instead of the kind defined in the configuration file. Options are: "+strings.Join(distribution.ConfigKinds(), ", "),
	)

	if err := upgradePathsCmd.RegisterFlagCompletionFunc("kind", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return distribution.ConfigKinds(), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	return upgradePathsCmd
}

func printUpgradePaths(kind, fromVersion string) error {
	out, err := FormatUpgradePaths(configs.Tpl, kind, fromVersion)
	if err != nil {
		return fmt.Errorf("error while computing the upgrade paths for version %s: %w", fromVersion, err)
	}

	if _, err := fmt.Fprint(os.Stdout, out); err != nil {
		return fmt.Errorf("error writing to stdout: %w", err)
	}

	return nil
}

// FormatUpgradePaths returns the human output of the upgrade paths of kind from version from, computed from
// the upgrades/<kind>/<from>-<to> folders of fsys: every folder is an edge of the upgrade graph.
func FormatUpgradePaths(fsys fs.FS, kind, from string) (string, error) {
	edges, err := readUpgradeGraph(fsys, strings.ToLower(kind))
	if err != nil {
		return "", err
	}

	direct := edges[from]
	if len(direct) == 0 {
		return fmt.Sprintf("No upgrade paths: %s is the latest version furyctl can upgrade to.\n", from), nil
	}

	slices.SortFunc(direct, compareVersions)

	var sb strings.Builder

	fmt.Fprintf(&sb, "Current version: %s (%s)\n\nDirect upgrades:\n", from, kind)

	const padWidth = 3

	w := tabwriter.NewWriter(&sb, 0, 0, padWidth, ' ', 0)

	var table strings.Builder

	table.WriteString("  TARGET\tRELEASE NOTES\n")

	for _, to := range direct {
		fmt.Fprintf(&table, "  %s\thttps://github.com/sighupio/distribution/blob/v%s/docs/releases/v%s.md\n", to, to, to)
	}

	if _, err := w.Write([]byte(table.String())); err != nil {
		return "", fmt.Errorf("error writing upgrade paths to table: %w", err)
	}

	if err := w.Flush(); err != nil {
		return "", fmt.Errorf("error writing upgrade paths to table: %w", err)
	}

	chain := pathToLatest(edges, from, direct[len(direct)-1])
	if !slices.Contains(direct, chain[len(chain)-1]) {
		hops := chain[1:]
		fmt.Fprintf(&sb, "\nPath to the latest version (%s), one upgrade at a time:\n  %s\n  Read the release notes of %s and %s before you start.\n",
			chain[len(chain)-1], strings.Join(chain, " -> "), strings.Join(hops[:len(hops)-1], ", "), hops[len(hops)-1])
	}

	fmt.Fprintf(&sb, "\nNext step: set spec.distributionVersion to v%s in furyctl.yaml and run furyctl apply --upgrade\n"+
		"Upgrade guide: https://docs.sighup.io/docs/installation/upgrades/\n", chain[1])

	return sb.String(), nil
}

func readUpgradeGraph(fsys fs.FS, kind string) (map[string][]string, error) {
	entries, err := fs.ReadDir(fsys, path.Join("upgrades", kind))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string][]string{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("error while reading the upgrade paths of kind %s: %w", kind, err)
	}

	edges := map[string][]string{}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		from, to, ok := strings.Cut(e.Name(), "-")
		_, fromErr := semver.NewVersion(from)
		_, toErr := semver.NewVersion(to)

		if !ok || fromErr != nil || toErr != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidUpgradeFolder, e.Name())
		}

		edges[from] = append(edges[from], to)
	}

	return edges, nil
}

// pathToLatest returns the shortest chain from "from" to the highest version reachable from it, taking at each
// hop the highest version still on a shortest chain. "from" is skipped because the search reaches it at hop 0:
// on a graph where every reachable version is lower than "from" the chain would otherwise have no hop.
func pathToLatest(edges map[string][]string, from, highestDirect string) []string {
	latest := highestDirect
	for v := range distances(edges, from) {
		if v != from && compareVersions(v, latest) > 0 {
			latest = v
		}
	}

	toLatest := distances(reverseEdges(edges), latest)
	chain := []string{from}

	for cur := from; cur != latest; {
		next := ""

		for _, n := range edges[cur] {
			if d, ok := toLatest[n]; ok && d == toLatest[cur]-1 && (next == "" || compareVersions(n, next) > 0) {
				next = n
			}
		}

		chain = append(chain, next)
		cur = next
	}

	return chain
}

// reverseEdges returns edges with every arrow flipped, so a breadth-first search from a version counts the hops
// needed to reach it.
func reverseEdges(edges map[string][]string) map[string][]string {
	reverse := map[string][]string{}

	for a, targets := range edges {
		for _, b := range targets {
			reverse[b] = append(reverse[b], a)
		}
	}

	return reverse
}

// distances runs a breadth-first search on edges from start and returns the hop count of every reached version.
func distances(edges map[string][]string, start string) map[string]int {
	hops := map[string]int{start: 0}

	for queue := []string{start}; len(queue) > 0; queue = queue[1:] {
		for _, n := range edges[queue[0]] {
			if _, seen := hops[n]; !seen {
				hops[n] = hops[queue[0]] + 1
				queue = append(queue, n)
			}
		}
	}

	return hops
}

// compareVersions compares versions already validated by readUpgradeGraph, so Must cannot panic.
func compareVersions(a, b string) int {
	return version.Must(semver.NewVersion(a)).Compare(version.Must(semver.NewVersion(b)))
}

// ReadUpgradePathsConfig reads only the kind and spec.distributionVersion of the configuration file in configPath.
func ReadUpgradePathsConfig(configPath string) (string, string, error) {
	furyctlConf, err := yamlx.FromFileV3[distroconf.Furyctl](configPath)
	if err != nil {
		return "", "", fmt.Errorf("error while reading configuration file in path %s: %w", configPath, err)
	}

	if furyctlConf.Kind == "" {
		return "", "", fmt.Errorf("%w: kind in %s", ErrUpgradePathsConfigField, configPath)
	}

	if furyctlConf.Spec.DistributionVersion == "" {
		return "", "", fmt.Errorf("%w: spec.distributionVersion in %s", ErrUpgradePathsConfigField, configPath)
	}

	return furyctlConf.Kind, furyctlConf.Spec.DistributionVersion, nil
}
