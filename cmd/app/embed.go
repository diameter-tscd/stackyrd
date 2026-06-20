package main

import (
	"stackyrd/pkg/assets"
	"stackyrd/pkg/infrastructure"
)

func init() {
	infrastructure.Init(assets.FS, map[string]string{
		"banner": "banner.txt",
	}, true)
}
