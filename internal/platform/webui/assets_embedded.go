//go:build webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedAssets embed.FS

func Assets() (fs.FS, error) {
	return fs.Sub(embeddedAssets, "dist")
}
