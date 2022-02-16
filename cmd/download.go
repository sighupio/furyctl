// Copyright (c) 2020 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"io/ioutil"
	"gopkg.in/yaml.v3"

	"github.com/sirupsen/logrus"

	getter "github.com/hashicorp/go-getter"
)

var parallel bool
var https bool
var prefix string

func download(packages []Package) error {

	// Preparing all the necessary data for a worker pool
	var wg sync.WaitGroup
	var numberOfWorkers int
	if parallel {
		numberOfWorkers = runtime.NumCPU() + 1
	} else {
		numberOfWorkers = 1
	}
	errChan := make(chan error, len(packages))
	jobs := make(chan Package, len(packages))
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
				res := get(data.url, data.dir, getter.ClientModeDir, true)
				errChan <- res
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

func get(src, dest string, mode getter.ClientMode, cleanGitFolder bool) error {

	logrus.Debugf("complete url downloading: %s -> %s", src, dest)

	var tempDest = dest + ".tmp"

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := &getter.Client{
		Src:  src,
		Dst:  tempDest,
		Pwd:  pwd,
		Mode: mode,
	}

	logrus.Debugf("let's get %s -> %s", src, dest)

	humanReadableDownloadLog(src, dest)

	err = client.Get()
	if err != nil {
		_ = removeDir(tempDest)
		return err
	}else{
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			logrus.Infof("%s already exists! removing it", dest)
			err = removeDir(dest)
			if err != nil {
				logrus.Error(err)
				return err
			}
		}

		err = renameDir(tempDest, dest)
		if err != nil {
			logrus.Error(err)
			return err
		}

	}

	if cleanGitFolder {
		gitFolder := fmt.Sprintf("%s/.git", dest)
		logrus.Infof("removing %s", gitFolder)
		err = removeDir(gitFolder)
	}

	if err != nil {
		logrus.Error(err)
		return err
	}

	logrus.Debugf("done %s -> %s", src, dest)

	return err
}


func mergeYAML(src, dest string, mode getter.ClientMode) error {

	logrus.Debugf("complete url downloading: %s -> %s", src, dest)

	var tempDest = dest + ".tmp"

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := &getter.Client{
		Src:  src,
		Dst:  tempDest,
		Pwd:  pwd,
		Mode: mode,
	}

	logrus.Debugf("let's get %s -> %s", src, dest)

	humanReadableDownloadLog(src, dest)

	err = client.Get()
	if err != nil {
		_ = removeDir(tempDest)
		return err
	}else{

		newFuryfile := map[string]interface{}{}
		currentFuryfile := map[string]interface{}{}

		// read one yaml file
		data, _ := ioutil.ReadFile(tempDest)
		if err := yaml.Unmarshal(data, &newFuryfile); err != nil {

		}

		// read another yaml file
		data1, _ := ioutil.ReadFile(dest)
		if err := yaml.Unmarshal(data1, &currentFuryfile); err != nil {

		}

		// merge both yaml data recursively
		currentFuryfile = deepMerge(currentFuryfile, newFuryfile)

		result, err := yaml.Marshal(currentFuryfile)
		if err != nil {
			logrus.Error(err)
			return err
		}

		err = ioutil.WriteFile(dest, result, 0644)
		if err != nil {
			logrus.Error(err)
			return err
		}

		if _, err := os.Stat(tempDest); !os.IsNotExist(err) {
			err = removeDir(tempDest)
			if err != nil {
				logrus.Error(err)
				return err
			}
		}

	}

	if err != nil {
		logrus.Error(err)
		return err
	}

	logrus.Debugf("done %s -> %s", src, dest)

	return err
}

// humanReadableDownloadLog prints a humanReadable log
func humanReadableDownloadLog(src string, dest string) {

	humanReadableSrc := src

	if strings.Count(src, "@") >= 1 {
		// handles git@github.com:sighupio url type
		humanReadableSrc = strings.Join(strings.Split(src, ":")[1:], ":")
		humanReadableSrc = strings.Replace(humanReadableSrc, "//", "/", 1)
	} else if strings.Count(humanReadableSrc, "//") >= 1 {
		// handles git::https://whatever.com//mymodule url type
		humanReadableSrc = strings.Join(strings.Split(humanReadableSrc, "//")[1:], "/")
	}

	logrus.Infof("downloading: %s -> %s", humanReadableSrc, dest)

}

func removeDir(dir string) error {
	err := os.RemoveAll(dir)
	if err != nil {
		return err
	}
	return nil
}

func renameDir(src string, dest string) error {
	err := os.Rename(src, dest)
	if err != nil {
		return err
	}
	return nil
}

func deepMerge(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = deepMerge(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}