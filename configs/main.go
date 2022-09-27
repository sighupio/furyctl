package configs

import "embed"

//go:embed furyctl.yaml.tpl
var Tpl embed.FS
