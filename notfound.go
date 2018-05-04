package main

import (
	"net/http"
)

// NotFoundHandler serves the "404" page
type NotFoundHandler struct{}

func (n NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serve as HTTP 200 to undo caching.
	http.Error(w, "File not found", 200)
}
