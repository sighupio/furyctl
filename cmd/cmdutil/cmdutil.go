package cmdutil

import (
	"errors"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	ErrNoOutputFormat      = errors.New("output cannot be nil")
	ErrNoConfigFileFound   = errors.New("no config files found")
	ErrUnknownOutputFormat = errors.New("unknown output format, supported ones are: json, text")
	ErrTooManyArguments    = errors.New("too many arguments")
)

func GetWd() string {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	return cwd
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
