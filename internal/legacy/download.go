// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legacy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/go-getter"
	"github.com/sirupsen/logrus"
)

const (
	httpsRepoPrefix         = "git::https://github.com/sighupio/fury-kubernetes"
	sshRepoPrefix           = "git@github.com:sighupio/fury-kubernetes"
	fallbackHTTPSRepoPrefix = "git::https://github.com/sighupio/kubernetes-fury"
	fallbackSSHRepoPrefix   = "git@github.com:sighupio/kubernetes-fury"
)

var (
	ErrDownloadRepo        = errors.New("error downloading repository")
	ErrSomeDownloadsFailed = errors.New("some downloads have failed. Please check the logs")
	ErrRemoveDir           = errors.New("error removing directory")
	ErrRenameDir           = errors.New("error renaming directory")
	ErrGettingWD           = errors.New("error getting working directory")
	ErrGETRequest          = errors.New("error performing GET request")
)

type Downloader struct {
	HTTPS bool
}

func NewDownloader(gitProtocol string) Downloader {
	return Downloader{
		HTTPS: gitProtocol == "https",
	}
}

func (d *Downloader) Download(packages []Package) error {
	var wg sync.WaitGroup

	errChan := make(chan error, len(packages))
	jobs := make(chan Package, len(packages))

	for _, p := range packages {
		jobs <- p
	}

	logrus.Debugf("workers = %d", len(jobs))

	for i, data := range packages {
		wg.Add(1)

		go d.downloadProcess(&wg, data, errChan, i)

		logrus.Debugf("created worker %d", i)
	}

	wg.Wait()

	close(jobs)

	close(errChan)

	logrus.Debugf("finished downloading all packages")

	if len(errChan) > 0 {
		for err := range errChan {
			if err != nil {
				errString := strings.ReplaceAll(err.Error(), "\n", " ")
				logrus.Errorln(errString)
			}
		}

		return ErrSomeDownloadsFailed
	}

	return nil
}

func (d *Downloader) downloadProcess(wg *sync.WaitGroup, data Package, errChan chan<- error, i int) {
	var pU *PackageURL

	var url string

	defer wg.Done()

	logrus.Debugf("worker %d : received data %v", i, data)

	if d.HTTPS {
		pU = newPackageURL(
			httpsRepoPrefix,
			strings.Split(data.Name, "/"),
			data.Kind,
			data.Version,
			data.Registry,
			data.ProviderOpt,
			data.ProviderKind)

		resp, err := checkRepository(pU)
		if err != nil {
			errChan <- err

			return
		}

		defer func() {
			if resp != nil && resp.Body != nil {
				if err := resp.Body.Close(); err != nil {
					logrus.Error(err)
				}
			}
		}()

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
			o := humanReadableSource(pU.getConsumableURL())

			pU.Prefix = fallbackHTTPSRepoPrefix

			logrus.Infof(
				"downloading '%s' failed, falling back to '%s' and retrying",
				o,
				humanReadableSource(pU.getConsumableURL()),
			)

			if resp, err := checkRepository(pU); err != nil ||
				resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
				errChan <- fmt.Errorf(
					"%w: error downloading %s for '%s' version '%s'. Both urls '%s' and '%s' have failed."+
						" Please check that the repository exists and that your credentials are"+
						" correctlly configured",
					err,
					data.Kind,
					data.Name,
					data.Version,
					o,
					humanReadableSource(pU.getConsumableURL()),
				)

				defer func() {
					if resp != nil && resp.Body != nil {
						if err := resp.Body.Close(); err != nil {
							logrus.Error(err)
						}
					}
				}()

				return
			}
		}

		url = pU.getConsumableURL()
		if token := os.Getenv("GITHUB_TOKEN"); token != "" && d.HTTPS {
			url = normalizeURLWithToken(pU.getConsumableURL())
		}
	}

	if !d.HTTPS {
		pU = newPackageURL(
			sshRepoPrefix,
			strings.Split(data.Name, "/"),
			data.Kind,
			data.Version,
			data.Registry,
			data.ProviderOpt,
			data.ProviderKind)

		url = pU.getConsumableURL()

		if err := get(url, data.Dir, getter.ClientModeDir); err != nil {
			o := humanReadableSource(pU.getConsumableURL())

			pU.Prefix = fallbackSSHRepoPrefix

			logrus.Infof(
				"downloading '%s' failed, falling back to %s and retrying",
				o,
				humanReadableSource(pU.getConsumableURL()),
			)

			url = pU.getConsumableURL()

			if err := get(url, data.Dir, getter.ClientModeDir); err != nil {
				errChan <- fmt.Errorf(
					"%w: error downloading %s for '%s' version '%s'. Both urls '%s' and '%s' have failed."+
						" Please check that the repository exists and that your credentials are"+
						" correctlly configured. You might want to try using the -H flag",
					err,
					data.Kind,
					data.Name,
					data.Version,
					o,
					humanReadableSource(pU.getConsumableURL()),
				)

				return
			}
		}
	}

	downloadErr := get(url, data.Dir, getter.ClientModeDir)
	if downloadErr != nil {
		if err := os.RemoveAll(data.Dir); err != nil {
			logrus.Errorf("error removing directory '%s': %s", data.Dir, err.Error())
		}
		errChan <- downloadErr
	}
}

func humanReadableSource(src string) string {
	humanReadableSrc := src

	if strings.Count(src, "@") >= 1 {
		humanReadableSrc = strings.Join(strings.Split(src, ":")[1:], ":")
		humanReadableSrc = strings.Replace(humanReadableSrc, "//", "/", 1)
	}

	if strings.Count(humanReadableSrc, "//") >= 1 {
		humanReadableSrc = strings.Join(strings.Split(humanReadableSrc, "//")[1:], "/")
	}

	return humanReadableSrc
}

func get(src, dest string, mode getter.ClientMode) error {
	logrus.Debugf("starting download process for '%s' into '%s'", src, dest)

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGettingWD, err)
	}

	client := &getter.Client{
		Src:  src,
		Dst:  dest + ".tmp",
		Pwd:  pwd,
		Mode: mode,
	}

	logrus.Debugf("downloading temporary file '%s' into '%s'", client.Src, client.Dst)

	h := humanReadableSource(src)

	logrus.Infof("downloading '%s' into '%s'", h, dest)

	if err := os.RemoveAll(client.Dst); err != nil {
		return fmt.Errorf("%w: %v", ErrRemoveDir, err)
	}

	if err := client.Get(); err != nil {
		return fmt.Errorf("%w: %v", ErrDownloadRepo, err)
	}

	if err := renameDir(client.Dst, dest); err != nil {
		return fmt.Errorf("%w: %v", ErrRenameDir, err)
	}

	gitFolder := dest + "/.git"
	logrus.Infof("removing git subfolder: %s", gitFolder)

	if err = os.RemoveAll(gitFolder); err != nil {
		return fmt.Errorf("%w: %v", ErrRemoveDir, err)
	}

	logrus.Debugf("download process finished: %s -> %s", src, dest)

	return nil
}

func renameDir(src, dest string) error {
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		logrus.Infof("removing existing folder: %s", dest)

		err = os.RemoveAll(dest)
		if err != nil {
			logrus.Error(err)

			return fmt.Errorf("%w: %s", ErrRemoveDir, dest)
		}
	}

	err := os.Rename(src, dest)
	if err != nil {
		return fmt.Errorf("%w: %s -> %s", ErrRenameDir, src, dest)
	}

	return nil
}

func normalizeURL(src string) string {
	var s string

	if strings.HasPrefix(src, "git@") {
		s = strings.Split(src, "//")[0]
		s = strings.Replace(s, "git@github.com:", "https://github.com/", 1)
	}

	if strings.HasPrefix(src, "git::") {
		_, s, _ = strings.Cut(src, "git::")
	}

	return strings.Split(s, ".git/")[0]
}

func normalizeURLWithAPI(src string) string {
	var s string

	if strings.HasPrefix(src, "git@") {
		s = strings.Split(src, "//")[0]
		s = strings.Replace(s, "git@github.com:", "https://api.github.com/repos/", 1)
	}

	if strings.HasPrefix(src, "git::") {
		s = strings.Replace(src, "git::https://github.com/", "https://api.github.com/repos/", 1)
	}

	return strings.Split(s, ".git/")[0]
}

func normalizeURLWithToken(src string) string {
	s := strings.Replace(src, "git::https://", "git::https://oauth2:"+os.Getenv("GITHUB_TOKEN")+"@", 1)

	return s
}

func checkRepository(pu *PackageURL) (*http.Response, error) {
	var url string

	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		url = normalizeURLWithAPI(pu.getConsumableURL())
	} else {
		url = normalizeURL(pu.getConsumableURL())
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGETRequest, err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	ghToken := os.Getenv("GITHUB_TOKEN")

	if ghToken != "" {
		req.Header.Set("Authorization", "Bearer "+ghToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGETRequest, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		url = normalizeURL(pu.getConsumableURL())

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGETRequest, err)
		}

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGETRequest, err)
		}
	}

	return resp, nil
}
