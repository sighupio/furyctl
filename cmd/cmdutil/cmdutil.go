package cmdutil

import (
	"errors"
	"os"

	"github.com/sirupsen/logrus"
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
		logrus.Fatal(err)
	}

	return cwd
}

func LoadConfig[T any](file string) T {
	var conf T

	configData, err := os.ReadFile(file)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := yaml.Unmarshal(configData, &conf); err != nil {
		logrus.Fatal(err)
	}

	return conf
}
