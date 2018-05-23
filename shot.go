package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oftn-oswg/ipcat"

	"github.com/oschwald/geoip2-golang"
)

var headerCacheControl = "no-cache, no-store, must-revalidate"

var sudo = &AdminClaims{Admin: true}

// ShotHandler serves the requested page and removes it from the database, or
// returns 404 page if not available.
type ShotHandler struct {
	App     *ZerodropApp
	Context *BlacklistContext
}

// NewShotHandler constructs a new ShotHandler from the arguments.
func NewShotHandler(app *ZerodropApp) *ShotHandler {
	config := app.Config

	ctx := &BlacklistContext{Databases: make(map[string]*ipcat.IntervalSet)}

	if config.GeoDB != "" {
		var err error
		ctx.GeoDB, err = geoip2.Open(config.GeoDB)
		if err != nil {
			log.Printf("Could not open geolocation database: %s", err.Error())
		}
	}

	for key, location := range config.IPCat {
		key = strings.ToLower(key)

		reader, err := os.Open(location)
		if err != nil {
			log.Printf("Could not open database %q: %s", key, err.Error())
			continue
		}

		ipset := ipcat.NewIntervalSet(4096)

		if err := ipset.ImportCSV(reader); err != nil {
			log.Printf("Could not import database %q: %s", key, err.Error())
			continue
		}

		ctx.Databases[key] = ipset
	}

	return &ShotHandler{
		App:     app,
		Context: ctx,
	}
}

// Access returns the ZerodropEntry with the specified name as long as access
// is permitted. The function returns nil otherwise.
func (a *ShotHandler) Access(name string, request *http.Request, redirectLevels int, direct bool) *ZerodropEntry {
	if name == "" {
		return nil
	}

	if redirectLevels <= 0 {
		log.Println("Exceeded redirection levels")
	}
	redirectLevels--

	if name == a.App.Config.SelfDestruct.Keyword {
		log.Println("Self destruct invoked")
		a.SelfDestruct()
		return nil
	}

	ip := RealRemoteIP(request)
	if ip == nil {
		log.Printf("Could not parse remote address from %s", request.RemoteAddr)
		return nil
	}

	entry, err := a.App.DB.Get(name)
	if err != nil {
		return nil
	}

	if entry.AccessTrain {
		date := time.Now().Format(time.RFC1123)
		entry.AccessBlacklist.Add(&BlacklistRule{Comment: "Automatically added by training on " + date})

		// We need to add the ip to the blacklist
		entry.AccessBlacklist.Add(&BlacklistRule{IP: ip})

		// We will also add the Geofence
		if a.Context.GeoDB != nil {
			record, err := a.Context.GeoDB.City(ip)
			if err == nil {
				entry.AccessBlacklist.Add(&BlacklistRule{
					Geofence: &Geofence{
						Latitude:  record.Location.Latitude,
						Longitude: record.Location.Longitude,
						Radius:    float64(record.Location.AccuracyRadius) * 1000.0, // Convert km to m
					},
				})
			}
		}

		if err := a.App.DB.Update(entry, sudo); err != nil {
			log.Printf("Error adding to blacklist: %s", err.Error())
		}
		return a.Access(entry.AccessRedirectOnDeny, request, redirectLevels, false)
	}

	if entry.IsExpired() {
		entry.AccessBlacklistCount++
		if err := a.App.DB.Update(entry, sudo); err != nil {
			log.Println(err)
		}
		return a.Access(entry.AccessRedirectOnDeny, request, redirectLevels, false)
	}

	if !entry.AccessBlacklist.Allow(a.Context, ip) {
		entry.AccessBlacklistCount++
		if err := a.App.DB.Update(entry, sudo); err != nil {
			log.Println(err)
		}
		return a.Access(entry.AccessRedirectOnDeny, request, redirectLevels, false)
	}

	entry.AccessCount++
	if err := a.App.DB.Update(entry, sudo); err != nil {
		log.Println(err)
	}

	return entry
}

// ServeHTTP generates the HTTP response.
func (a *ShotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get entry
	name := strings.Trim(r.URL.Path, "/")
	if name == "" {
		http.Redirect(w, r, "/admin/", 308)
		return
	}

	entry := a.Access(name, r, a.App.Config.RedirectLevels, true)

	ip := RealRemoteIP(r)

	if entry == nil {
		log.Printf("Denied access to %s to %s", strconv.Quote(name), ip)
		a.App.NotFound.ServeHTTP(w, r)
		return
	}

	log.Printf("Granted access to %s to %s", strconv.Quote(entry.Name), ip)

	// File Upload
	if entry.URL == "" {
		contentType := entry.ContentType
		if contentType == "" {
			contentType = "text/plain"
		}

		fullpath := filepath.Join(a.App.Config.UploadDirectory, entry.Filename)
		file, err := os.Open(fullpath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", headerCacheControl)
		io.Copy(w, file)
		return
	}

	// URL redirect
	if entry.Redirect {
		// Perform a redirect to the URL.
		w.Header().Set("Cache-Control", headerCacheControl)
		http.Redirect(w, r, entry.URL, 307)
		return
	}

	// URL proxy
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
			w.Header().Set("Cache-Control", headerCacheControl)
			return nil
		},
	}

	proxy.ServeHTTP(w, r)
}

func (a *ShotHandler) SelfDestruct() {
	a.App.SelfDestruct()
}
