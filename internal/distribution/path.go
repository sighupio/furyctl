package distribution

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/config"
)

func GetSchemaPath(basePath string, conf config.Furyctl) (string, error) {
	avp := strings.Split(conf.ApiVersion, "/")

	if len(avp) < 2 {
		return "", fmt.Errorf("invalid apiVersion: %s", conf.ApiVersion)
	}

	ns := strings.Replace(avp[0], ".sighup.io", "", 1)
	ver := avp[1]

	if conf.Kind == "" {
		return "", fmt.Errorf("kind is empty")
	}

	filename := fmt.Sprintf("%s-%s-%s.json", strings.ToLower(conf.Kind), ns, ver)

	return filepath.Join(basePath, "schemas", filename), nil
}

func GetDefaultsPath(basePath string) string {
	return filepath.Join(basePath, "furyctl-defaults.yaml")
}
