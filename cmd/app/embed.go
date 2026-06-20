package main

import "stackyrd-nano/pkg/assets"

func init() {
	data, err := assets.FS.ReadFile("banner.txt")
	if err == nil {
		embeddedBanner = string(data)
	}
}

var embeddedBanner string
