package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// ShotHandler serves the requested page and removes it from the database, or
// returns 404 page if not available.
type ShotHandler struct {
	DB       *ZerodropDB
	Config   *ZerodropConfig
	NotFound NotFoundHandler
}

func (a *ShotHandler) DetermineAccess(entry *ZerodropEntry, ip net.IP) bool {
	if entry.AccessTrain {
		// We need to add the ip to the blacklist
		bits := len(ip) * 8
		item := &ZerodropBlacklistItem{
			Network: &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(bits, bits),
			},
		}
		entry.AccessBlacklist.Add(item)
		if err := entry.Update(); err != nil {
			log.Printf("Error adding to blacklist: %s", err.Error())
			return false
		}

		log.Printf("Added %s to blacklist of %s", item, entry.Name)
		return false
	}

	if entry.IsExpired() {
		log.Printf("Access restricted to expired %s from %s", entry.Name, ip.String())
		return false
	}

	if !entry.AccessBlacklist.Allow(ip) {
		log.Printf("Access restricted to %s from blacklisted %s", entry.Name, ip.String())
		return false
	}

	return true
}

// ServeHTTP generates the HTTP response.
func (a *ShotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get requester information
	host, _, err := net.SplitHostPort(RealRemoteAddr(r))
	ip := net.ParseIP(host)

	// Get entry
	name := strings.Trim(r.URL.Path, "/")
	entry, ok := a.DB.Get(name)
	if !ok || ip == nil || !a.DetermineAccess(&entry, ip) {
		a.NotFound.ServeHTTP(w, r)
		return
	}

	// Access entry
	if err := entry.Access(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	log.Println("Access to " + r.URL.Path + " granted to IP " + host)

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
