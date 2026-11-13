// Package server is the web application: it serves the embedded interface.
package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const indexPage = "index.html"

// New returns the handler of the web application. assets is the compiled
// interface, with index.html at its root.
//
// The interface is a single-page application, so any path that is not a file
// gets index.html and the router in the browser takes over. A path with a file
// extension that does not exist is a real 404: answering it with HTML would
// hide a broken asset reference.
func New(assets fs.FS) http.Handler {
	files := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

		if info, err := fs.Stat(assets, name); name != "" && err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, assets, indexPage)
	})
}
