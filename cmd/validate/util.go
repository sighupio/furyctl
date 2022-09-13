package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/schema/santhosh"
)

var (
	ErrHasValidationErrors = errors.New("schema has validation errors")
	ErrUnknownOutputFormat = errors.New("unknown output format")
)

type FuryDistributionSpecs struct {
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

func LoadConfig[T any](file string) T {
	var conf T

	configData, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	if err := yaml.Unmarshal(configData, &conf); err != nil {
		log.Fatal(err)
	}

	return conf
}

func PrintSummary(output string, hasErrors bool) {
	switch output {
	case "text":
		if hasErrors {
			fmt.Println("Validation failed")
		} else {
			fmt.Println("Validation succeeded")
		}

	case "json":
		if hasErrors {
			fmt.Println("{\"result\":\"Validation failed\"}")
		} else {
			fmt.Println("{\"result\":\"Validation succeeded\"}")
		}

	default:
		log.Fatal(fmt.Errorf("'%s': %w", output, ErrUnknownOutputFormat))
	}
}

func PrintResults(output string, err error, conf any, configFile string) {
	ptrPaths := santhosh.GetPtrPaths(err)

	switch output {
	case "text":
		printTextResults(ptrPaths, err, conf, configFile)

	case "json":
		printJSONResults(ptrPaths, err, conf, configFile)

	default:
		log.Fatal(fmt.Errorf("'%s': %w", output, ErrUnknownOutputFormat))
	}
}

func printTextResults(ptrPaths [][]any, err error, conf any, configFile string) {
	fmt.Printf("CONFIG FILE %s\n", configFile)

	for _, path := range ptrPaths {
		value, serr := santhosh.GetValueAtPath(conf, path)
		if serr != nil {
			log.Fatal(serr)
		}

		fmt.Printf(
			"path '%s' contains an invalid configuration value: %+v\n",
			santhosh.JoinPtrPath(path),
			value,
		)
	}

	fmt.Println(err)
}

func printJSONResults(ptrPaths [][]any, err error, conf any, configFile string) {
	for _, path := range ptrPaths {
		value, serr := santhosh.GetValueAtPath(conf, path)
		if serr != nil {
			log.Fatal(serr)
		}

		jv, jerr := json.Marshal(
			map[string]any{
				"file":    configFile,
				"message": "path contains an invalid configuration value",
				"path":    santhosh.JoinPtrPath(path),
				"value":   value,
			},
		)
		if jerr != nil {
			log.Fatal(jerr)
		}

		fmt.Println(jv)
	}

	jv, jerr := json.Marshal(err)
	if jerr != nil {
		log.Fatal(jerr)
	}

	fmt.Println(jv)
}
