// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
)

var (
	fallbackHttpsRepoPrefix = "git::https://github.com/sighupio/fury-kubernetes"
	fallbackSshRepoPrefix   = "git@github.com:sighupio/fury-kubernetes"
	httpsRepoPrefix         = "git::https://github.com/sighupio/kubernetes-fury"
	sshRepoPrefix           = "git@github.com:sighupio/kubernetes-fury"
)

type DownloadOpts struct {
	Parallel bool
	Https    bool
	Fallback bool
}

//PackageURL is the representation of the fields needed to elaborate a url
type PackageURL struct {
	Prefix        string
	Blocks        []string
	Kind          string
	Version       string
	Registry      bool
	CloudProvider ProviderOptSpec
	KindSpec      ProviderKind
}

// newUrl initialize the PackageURL struct
func newPackageURL(prefix string, blocks []string, kind, version string, registry bool, cloud ProviderOptSpec, kindSpec ProviderKind) *PackageURL {
	return &PackageURL{
		Prefix:        prefix,
		Registry:      registry,
		Blocks:        blocks,
		Kind:          kind,
		Version:       version,
		CloudProvider: cloud,
		KindSpec:      kindSpec,
	}
}

//getConsumableURL returns an url that can be used for download
func (n *PackageURL) getConsumableURL() string {

	if !n.Registry {
		return n.getURLFromCompanyRepos()
	}

	return fmt.Sprintf("%s/%s%s?ref=%s", n.KindSpec.pickCloudProviderURL(n.CloudProvider), n.Blocks[0], ".git", n.Version)

}

func (n *PackageURL) getURLFromCompanyRepos() string {
	if len(n.Blocks) == 0 {
		// todo should return error?
		return ""
	}

	dG := ""

	if strings.HasPrefix(n.Prefix, "git::https") {
		dG = ".git"
	}

	if len(n.Blocks) == 1 {
		return fmt.Sprintf("%s-%s%s//%s?ref=%s", n.Prefix, n.Blocks[0], dG, n.Kind, n.Version)
	}
	// always len(n.Blocks) >= 2 {
	var remainingBlocks string

	for i := 1; i < len(n.Blocks); i++ {
		remainingBlocks = path.Join(remainingBlocks, n.Blocks[i])
	}

	return fmt.Sprintf("%s-%s%s//%s/%s?ref=%s", n.Prefix, n.Blocks[0], dG, n.Kind, remainingBlocks, n.Version)

}

func Download(opts DownloadOpts, pkgs []Package) error {
	if opts.Parallel {
		return parallelDownload(pkgs, opts)
	}

	return download(pkgs, opts)
}

func parallelDownload(packages []Package, opts DownloadOpts) error {
	//Preparing all the necessary data for a worker pool
	var wg sync.WaitGroup
	errChan := make(chan error, len(packages))
	jobs := make(chan Package, len(packages))

	numberOfWorkers := runtime.NumCPU() + 1

	logrus.Debugf("workers = %d", numberOfWorkers)

	// Populating the job channel with all the packages to downlaod
	for _, p := range packages {
		jobs <- p
	}

	// Starting all the workers necessary
	for i := 0; i < numberOfWorkers; i++ {
		wg.Add(1)
		go func(i int) {
			for data := range jobs {
				logrus.Debugf("%d : received data %v", i, data)
				p := ""

				if opts.Https {
					p = httpsRepoPrefix
				} else {
					p = sshRepoPrefix
				}

				pU := newPackageURL(
					p,
					strings.Split(data.Name, "/"),
					data.Kind,
					data.Version,
					data.Registry,
					data.ProviderOpt,
					data.ProviderKind)

				u := pU.getConsumableURL()

				err := get(u, data.Dir, getter.ClientModeDir, true)

				if err != nil && strings.Contains(err.Error(), "Repository not found") {
					o := humanReadableSource(pU.getConsumableURL())

					if opts.Https {
						pU.Prefix = fallbackHttpsRepoPrefix
					} else {
						pU.Prefix = fallbackSshRepoPrefix
					}

					logrus.Warningf("error downloading %s, falling back to %s", o, humanReadableSource(pU.getConsumableURL()))

					err = get(pU.getConsumableURL(), data.Dir, getter.ClientModeDir, true)
					if err != nil {
						logrus.Error(err.Error())
					}
				}

				errChan <- err
				logrus.Debugf("%d : finished with data %v", i, data)
			}
			logrus.Debugf("%d : CLOSING", i)
			wg.Done()
		}(i)
		logrus.Debugf("created worker %d", i)
	}

	close(jobs)
	logrus.Debugf("closed jobs")
	wg.Wait()
	close(errChan)
	logrus.Debugf("finished")
	for err := range errChan {
		if err != nil {
			//todo ISSUE: logrus doesn't escape string characters
			errString := strings.Replace(err.Error(), "\n", " ", -1)
			logrus.Errorln(errString)
		}
	}
	return nil
}

func download(packages []Package, opts DownloadOpts) (downloadErr error) {
	var repoPrefix string

	if opts.Https {
		repoPrefix = httpsRepoPrefix
	} else {
		repoPrefix = sshRepoPrefix
	}

	for _, p := range packages {
		pU := newPackageURL(
			repoPrefix,
			strings.Split(p.Name, "/"),
			p.Kind,
			p.Version,
			p.Registry,
			p.ProviderOpt,
			p.ProviderKind)

		u := pU.getConsumableURL()

		downloadErr = get(u, p.Dir, getter.ClientModeDir, true)
		if downloadErr != nil && strings.Contains(downloadErr.Error(), "remote: Repository not found.") {
			o := humanReadableSource(pU.getConsumableURL())

			if opts.Https {
				pU.Prefix = fallbackHttpsRepoPrefix
			} else {
				pU.Prefix = fallbackSshRepoPrefix
			}

			logrus.Warningf("error downloading %s, falling back to %s", o, humanReadableSource(pU.getConsumableURL()))

			downloadErr = get(pU.getConsumableURL(), p.Dir, getter.ClientModeDir, true)
			if downloadErr != nil {
				logrus.Error(downloadErr.Error())
			}
		}
	}

	return downloadErr
}

func get(src, dest string, mode getter.ClientMode, cleanGitFolder bool) error {
	logrus.Debugf("starting download process: %s -> %s", src, dest)

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := &getter.Client{
		Src:  src,
		Dst:  dest + ".tmp",
		Pwd:  pwd,
		Mode: mode,
	}

	logrus.Debugf("downloading temporary file: %s -> %s", client.Src, client.Dst)

	h := humanReadableSource(src)

	logrus.Infof("downloading: %s -> %s", h, dest)

	if err := removeDir(client.Dst); err != nil {
		logrus.Errorf("failed to remove: %s", client.Dst)
		return err
	}

	if err := client.Get(); err != nil {
		removeDir(client.Dst)

		return err
	}

	if err := renameDir(client.Dst, dest); err != nil {
		logrus.Error(err)
		return err
	}

	if cleanGitFolder {
		gitFolder := fmt.Sprintf("%s/.git", dest)
		logrus.Infof("cleaning git subfolder: %s", gitFolder)
		if err := removeDir(gitFolder); err != nil {
			logrus.Error(err)
			return err
		}
	}

	logrus.Debugf("download process finished: %s -> %s", src, dest)

	return err
}

// humanReadableSource returns a human-readable string for the given source
func humanReadableSource(src string) (humanReadableSrc string) {
	humanReadableSrc = src

	if strings.Count(src, "@") >= 1 {
		// handles git@github.com:sighupio url type
		humanReadableSrc = strings.Join(strings.Split(src, ":")[1:], ":")
		humanReadableSrc = strings.Replace(humanReadableSrc, "//", "/", 1)
	}

	if strings.Count(humanReadableSrc, "//") >= 1 {
		// handles git::https://whatever.com//mymodule url type
		humanReadableSrc = strings.Join(strings.Split(humanReadableSrc, "//")[1:], "/")
	}

	return humanReadableSrc
}

func removeDir(dir string) error {
	return os.RemoveAll(dir)
}

func renameDir(src string, dest string) error {
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		logrus.Infof("removing target path: %s", dest)
		err = removeDir(dest)
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
	err := os.Rename(src, dest)
	if err != nil {
		return err
	}
	return nil
}
