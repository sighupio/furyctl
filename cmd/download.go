// Copyright (c) 2022 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
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

// PackageURL is the representation of the fields needed to elaborate an url
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

// getConsumableURL returns an url that can be used for download
func (n *PackageURL) getConsumableURL() string {

	if !n.Registry {
		return n.getURLFromCompanyRepos()
	}

	return fmt.Sprintf("%s/%s%s?ref=%s", n.KindSpec.pickCloudProviderURL(n.CloudProvider), n.Blocks[0], ".git", n.Version)

}

func (n *PackageURL) getURLFromCompanyRepos() string {
	if len(n.Blocks) == 0 {
		return ""
	}

	dG := ""

	if strings.HasPrefix(n.Prefix, "git::https") {
		dG = ".git"
	}

	if len(n.Blocks) == 1 {
		return fmt.Sprintf("%s-%s%s//%s?ref=%s", n.Prefix, n.Blocks[0], dG, n.Kind, n.Version)
	}

	remainingBlocks := ""

	for i := 1; i < len(n.Blocks); i++ {
		remainingBlocks = path.Join(remainingBlocks, n.Blocks[i])
	}

	return fmt.Sprintf("%s-%s%s//%s/%s?ref=%s", n.Prefix, n.Blocks[0], dG, n.Kind, remainingBlocks, n.Version)

}

func downloadProcess(wg *sync.WaitGroup, opts DownloadOpts, data Package, errChan chan<- error, i int) {
	// deferring the worker to be done
	defer wg.Done()

	logrus.Debugf("%d : received data %v", i, data)

	// Checking git clone protocol
	p := sshRepoPrefix

	if opts.Https {
		p = httpsRepoPrefix
	}

	// Create the package URL from the data received to download the package
	pU := newPackageURL(
		p,
		strings.Split(data.Name, "/"),
		data.Kind,
		data.Version,
		data.Registry,
		data.ProviderOpt,
		data.ProviderKind)

	u := pU.getConsumableURL()

	resp, err := http.Get(fromSshToHttps(u))
	if err != nil {
		errChan <- err
		return
	}

	if opts.Https {
		p = httpsRepoPrefix
	}

	if resp.StatusCode == 404 {
		// Checking if repository was found otherwise fallback to the old prefix, if fallback fails sends error to tehe error channel
		o := humanReadableSource(pU.getConsumableURL())

		if opts.Https {
			pU.Prefix = fallbackHttpsRepoPrefix
		} else {
			pU.Prefix = fallbackSshRepoPrefix
		}

		logrus.Infof("error downloading %s, falling back to %s", o, humanReadableSource(pU.getConsumableURL()))

		if resp, err := http.Get(fromSshToHttps(o)); err != nil || resp.StatusCode == 404 {
			errChan <- fmt.Errorf("Unable to download %s. Please check repository exists or if your credentials are correctlly configured", o)
			return
		}
	}

	downloadErr := get(pU.getConsumableURL(), data.Dir, getter.ClientModeDir, true)
	if downloadErr != nil {
		if err := os.RemoveAll(data.Dir); err != nil {
			logrus.Errorf("error removing directory %s: %s", data.Dir, err.Error())
		}
		errChan <- downloadErr
	}
}

func Download(packages []Package, opts DownloadOpts) error {
	//Preparing all the necessary data for a worker pool
	var wg sync.WaitGroup
	errChan := make(chan error, len(packages))
	jobs := make(chan Package, len(packages))

	// Populating the job channel with all the packages to downlaod
	for _, p := range packages {
		jobs <- p
	}

	logrus.Debugf("workers = %d", len(jobs))

	// Starting the workers to download the packages in parallel
	for i, data := range packages {
		wg.Add(1)

		go downloadProcess(&wg, opts, data, errChan, i)

		logrus.Debugf("created worker %d", i)
	}

	// Waiting for all the workers to finish
	wg.Wait()

	// Closing the job channel
	close(jobs)

	// Closing the error channel
	close(errChan)

	logrus.Debugf("finished downloading all packages")

	// Checking if there was any error during the download, if so, print it
	if len(errChan) > 0 {
		for err := range errChan {
			if err != nil {
				errString := strings.Replace(err.Error(), "\n", " ", -1)
				logrus.Errorln(errString)
			}
		}

		return errors.New("download failed. See the logs")
	}

	return nil
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

	if err := os.RemoveAll(client.Dst); err != nil {
		return err
	}

	if err := client.Get(); err != nil {
		return err
	}

	if err := renameDir(client.Dst, dest); err != nil {
		return err
	}

	if cleanGitFolder {
		gitFolder := fmt.Sprintf("%s/.git", dest)
		logrus.Infof("cleaning git subfolder: %s", gitFolder)
		if err = os.RemoveAll(gitFolder); err != nil {
			return err
		}
	}

	logrus.Debugf("download process finished: %s -> %s", src, dest)

	return nil
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

func renameDir(src string, dest string) error {
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		logrus.Infof("removing target path: %s", dest)
		err = os.RemoveAll(dest)
		if err != nil {
			logrus.Error(err)
			return err
		}
	}

	return os.Rename(src, dest)
}

func fromSshToHttps(src string) string {
	s := strings.Replace(src, "git@github.com:", "https://github.com/", 1)

	if strings.Contains(s, "git::") {
		_, s, _ = strings.Cut(s, "git::")
	}

	return s
}
