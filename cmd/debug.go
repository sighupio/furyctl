package cmd

import (
	"log"
	"runtime"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

func LogIfDebugOn(printf string, arguments ...interface{}) {
	debug, err := rootCmd.Flags().GetBool("debug")

	if err != nil {
		log.Fatal("debug error")
	}

	if !debug {
		return
	}

	if runtime.GOOS != "windows" {
		log.Println(colorCyan)
	}

	log.Printf(printf, arguments...)

	if runtime.GOOS != "windows" {
		log.Println(colorCyan)
	}
}
