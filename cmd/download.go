package cmd

import (
	"log"
	"os"
	"runtime"
	"sync"

	getter "github.com/hashicorp/go-getter"
)

var parallel bool

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
	//log.Printf("workers = %d", numberOfWorkers)

	// Populating the job channel with all the packages to downlaod
	for _, p := range packages {
		jobs <- p
	}

	// Starting all the workers necessary
	for i := 0; i < numberOfWorkers; i++ {
		wg.Add(1)
		go func(i int) {
			for data := range jobs {
				//log.Printf("%d : received data %v", i, data)
				res := get(data.url, data.dir)
				errChan <- res
				//log.Printf("%d : finished with data %v", i, data)
			}
			//log.Printf("%d : CLOSING", i)
			wg.Done()
		}(i)
		//log.Printf("created worker %d", i)
	}

	close(jobs)
	//log.Print("closed jobs")
	wg.Wait()
	close(errChan)
	//log.Print("finished")
	for err := range errChan {
		if err != nil {
			log.Print(err)
		}
	}
	return nil
}

func get(src, dest string) error {
	log.Printf("downloading: %s -> %s\n", src, dest)
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	client := &getter.Client{
		Src:  src,
		Dst:  dest,
		Pwd:  pwd,
		Mode: getter.ClientModeDir,
	}
	//log.Printf("let's get %s -> %s", src, dest)
	err = client.Get()
	//log.Printf("done %s -> %s", src, dest)
	return err
}
