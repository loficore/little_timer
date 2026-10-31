//go:build embed_ui && !android

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:assets
var embeddedAssets embed.FS

func embeddedUIFS() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil
	}
	return sub
}
