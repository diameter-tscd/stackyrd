package assets

import "embed"

//go:embed banner.txt config.yaml
var FS embed.FS
