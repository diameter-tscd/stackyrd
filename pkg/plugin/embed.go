package plugin

import (
	"embed"
)

//go:embed builtin
var pluginBuiltinFS embed.FS

func init() {
	SetBuiltinFS(pluginBuiltinFS)
}
