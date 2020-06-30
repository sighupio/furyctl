package cmd

import (
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	getter "github.com/hashicorp/go-getter"
)

var parallel bool
var https bool

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
				res := get(data.url, data.dir, getter.ClientModeDir)
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
			log.Print(err)
		}
	}
	return nil
}

func get(src, dest string, mode getter.ClientMode) error {

	logrus.Debugf("complete url downloading: %s -> %s\n", src, dest)

	humanReadableDownloadLog(src, dest)

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	client := &getter.Client{
		Src:  src,
		Dst:  dest,
		Pwd:  pwd,
		Mode: mode,
	}
	logrus.Debugf("let's get %s -> %s", src, dest)
	err = client.Get()
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

	log.Printf("downloading: %s -> %s\n", humanReadableSrc, dest)

}
