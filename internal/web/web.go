package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html style.css app.js
var webFS embed.FS

// GetFS returns the fs.FS for the webview frontend static assets.
func GetFS() fs.FS {
	return webFS
}
