package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns an fs.FS sub-filesystem representing the dist directory.
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// HasEmbedded returns true if the embedded frontend contains index.html.
func HasEmbedded() bool {
	f, err := distFS.Open("dist/index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
