package webui

import (
	"embed"
	"io/fs"
)

// Dist holds the compiled frontend. During development it contains only a
// placeholder; the Vite build overwrites it with the real assets.
//
//go:embed all:dist
var Dist embed.FS

// FS returns the frontend build output rooted at the dist directory.
func FS() (fs.FS, error) {
	return fs.Sub(Dist, "dist")
}
