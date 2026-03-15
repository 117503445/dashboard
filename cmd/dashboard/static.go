package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var staticFS embed.FS

func staticHandler() http.Handler {
	// Get the dist subdirectory from the embedded FS
	distFS, err := fs.Sub(staticFS, "dist")
	if err != nil {
		panic(err)
	}

	// Create a file server for the static files
	fileServer := http.FileServer(http.FS(distFS))

	// Wrap to handle SPA routing - serve index.html for non-file paths
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		if _, err := distFS.Open(path[1:]); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// For SPA, serve index.html for unmatched routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}