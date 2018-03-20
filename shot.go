package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
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
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.Redirect(w, r, entry.URL, 307)
		return
	}

	// Perform a proxying
	target, err := url.Parse(entry.URL)
	if err != nil {
		http.Error(w, "Could not parse URL", 500)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target
			req.Host = target.Host
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
		},
		ModifyResponse: func(res *http.Response) error {
			res.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			return nil
		},
	}

	proxy.ServeHTTP(w, r)
}
