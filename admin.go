package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AdminHandler serves the administration page, or asks for credentials if not
// already authenticated.
type AdminHandler struct {
	DB        *ZerodropDB
	Config    *ZerodropConfig
	Templates *template.Template
	Mux       *http.ServeMux
}

// NewAdminHandler creates a new admin handler with the specified configuration
// and loads the template files into cache.
func NewAdminHandler(db *ZerodropDB, config *ZerodropConfig) *AdminHandler {
	handler := &AdminHandler{DB: db, Config: config}

	// Load templates
	var allFiles []string
	files, err := ioutil.ReadDir("./templates")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		filename := file.Name()
		if strings.HasSuffix(filename, ".tmpl") {
			allFiles = append(allFiles, "./templates/"+filename)
		}
	}

	handler.Templates, err = template.ParseFiles(allFiles...)
	if err != nil {
		log.Fatal(err)
	}

	// Create ServeMux
	handler.Mux = http.NewServeMux()
	handler.Mux.HandleFunc("/new", handler.ServeNew)
	handler.Mux.HandleFunc("/", handler.ServeMain)

	return handler
}

type AdminPageData struct {
	Error    string
	Title    string
	LoggedIn bool
	Config   *ZerodropConfig
	Entries  []ZerodropEntry
}

func (a *AdminHandler) ServeLogin(w http.ResponseWriter, r *http.Request) {
	page := a.Config.Base + "admin/login"
	if r.URL.Path != "/login" {
		http.Redirect(w, r, page, 302)
		return
	}

	data := &AdminPageData{Title: "Zerodrop Login", Config: a.Config}
	loginTmpl := a.Templates.Lookup("admin-login.tmpl")
	err := loginTmpl.ExecuteTemplate(w, "login", data)
	if err != nil {
		log.Println(err)
	}
}

func (a *AdminHandler) ServeNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()

		creation := time.Now()

		// Source information
		url := r.FormValue("url")
		redirect := r.FormValue("url_type") == "redirect"

		// Access information
		accessExpire := r.FormValue("access_expire") != ""
		accessExpireCount, err := strconv.Atoi(r.FormValue("access_expire_count"))
		if err != nil {
			accessExpireCount = 0
		}
		accessBlacklist := r.FormValue("access_blacklist")

		// Publish information
		publish := r.FormValue("publish")

		entry := &ZerodropEntry{
			Name:              publish,
			URL:               url,
			Redirect:          redirect,
			Creation:          creation,
			AccessBlacklist:   accessBlacklist,
			AccessExpire:      accessExpire,
			AccessExpireCount: accessExpireCount,
		}

		http.Redirect(w, r, a.Config.Base+"admin/", 302)
		return
	}

	data := &AdminPageData{Title: "Zerodrop New", LoggedIn: true, Config: a.Config}
	loginTmpl := a.Templates.Lookup("admin-new.tmpl")
	err := loginTmpl.ExecuteTemplate(w, "new", data)
	if err != nil {
		log.Println(err)
	}
}

func (a *AdminHandler) ServeMain(w http.ResponseWriter, r *http.Request) {
	data := &AdminPageData{Title: "Zerodrop Admin", LoggedIn: true, Config: a.Config}

	if r.Method == "POST" {
		r.ParseForm()

		switch r.FormValue("action") {
		case "add":
			accessExpiry, err := strconv.Atoi(r.FormValue("access_expiry"))
			if err != nil {
				accessExpiry = 0
			}

			entry := &ZerodropEntry{
				URL:          r.FormValue("url"),
				Redirect:     r.FormValue("redirect") != "",
				Creation:     time.Now(),
				AccessExpiry: accessExpiry,
				AccessCount:  0,
			}

			if err := a.DB.Create(entry); err == nil {
				log.Printf("Added entry: %#v", entry)
			}

		case "delete":
			uuid := r.FormValue("uuid")
			if uuid != "" {
				a.DB.Remove(uuid)
				log.Printf("Removed entry: %s", uuid)
			}

		case "clear":
			a.DB.Clear()
			log.Println("Cleared all entries")

		}

		http.Redirect(w, r, a.Config.Base+"admin/", 302)
		return
	}

	data.Entries = a.DB.List()

	interfaceTmpl := a.Templates.Lookup("admin-main.tmpl")
	err := interfaceTmpl.ExecuteTemplate(w, "main", data)
	if err != nil {
		log.Println(err)
	}
}

func (a *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Access to " + r.URL.Path + " granted to IP " + RealRemoteAddr(r))
	a.Mux.ServeHTTP(w, r)
}
