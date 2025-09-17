// Package web provides embedded web assets for Watchdog.
package web

import (
	"io/fs"
	"net/http"
	"os"
)

// StaticFS returns the embedded static file system.
// Returns a minimal filesystem if no static files are embedded.
func StaticFS() http.FileSystem {
	return http.Dir("web/static")
}

// TemplatesFS returns the embedded templates file system.
// Returns the local filesystem if no templates are embedded.
func TemplatesFS() fs.FS {
	return os.DirFS("web/templates")
}

// StaticHandler returns an HTTP handler for static files.
func StaticHandler() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(StaticFS()))
}
