package main

import (
	"io"
	"log"
	"net/http"
	"strings"
)

// ShotHandler serves the requested page and removes it from the database, or
// returns 404 page if not available.
type ShotHandler struct {
	DB       *OneshotDB
	Config   *OneshotConfig
	NotFound NotFoundHandler
}

// ServeHTTP generates the HTTP response.
func (a *ShotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uuid := strings.Trim(r.URL.Path, "/")

	entry, ok := a.DB.Access(uuid)
	if !ok || entry.IsExpired() {
		a.NotFound.ServeHTTP(w, r)
		return
	}

	if entry.Redirect {
		// Perform a redirect to the URL.
		http.Redirect(w, r, entry.URL, 302)
		return
	}

	// Perform a proxying
	response, err := http.Get(entry.URL)
	if err != nil {
		log.Println("Error serving " + uuid + ": " + err.Error())
		a.NotFound.ServeHTTP(w, r)
		return
	}
	go func() {
		_, err := io.Copy(w, response.Body)
		if err != nil {
			log.Println(err)
		}
	}()
}
