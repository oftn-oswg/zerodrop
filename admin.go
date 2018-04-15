package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type AdminClaims struct {
	Admin bool `json:"admin"`
	jwt.StandardClaims
}

// AdminHandler serves the administration page, or asks for credentials if not
// already authenticated.
type AdminHandler struct {
	DB        *ZerodropDB
	Config    *ZerodropConfig
	Templates *template.Template
}

// NewAdminHandler creates a new admin handler with the specified configuration
// and loads the template files into cache.
func NewAdminHandler(db *ZerodropDB, config *ZerodropConfig) *AdminHandler {
	handler := &AdminHandler{DB: db, Config: config}

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

	return handler
}

type AdminPageData struct {
	Error   string
	Title   string
	Config  *ZerodropConfig
	Entries []ZerodropEntry
}

func (a *AdminHandler) ServeLogin(w http.ResponseWriter, r *http.Request, data *AdminPageData) {
	loginTmpl := a.Templates.Lookup("admin-login.tmpl")
	err := loginTmpl.ExecuteTemplate(w, "login", data)
	if err != nil {
		log.Println(err)
	}
}

func (a *AdminHandler) ServeInterface(w http.ResponseWriter, r *http.Request) {
	log.Println("Access to " + r.URL.Path + " granted to IP " + RealRemoteAddr(r))

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

	data := &AdminPageData{}
	data.Title = "Zerodrop Admin"
	data.Config = a.Config
	data.Entries = a.DB.List()

	interfaceTmpl := a.Templates.Lookup("admin-interface.tmpl")
	err := interfaceTmpl.ExecuteTemplate(w, "admin", data)
	if err != nil {
		log.Println(err)
	}
}

// ServeHTTP generates the HTTP response.
func (a *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := &AdminPageData{}
	data.Title = "Zerodrop Login"
	data.Config = a.Config

	// Verify authentication
	ok, err := VerifyRequestAuth(r, a.Config.AuthSecret)
	if err != nil {
		data.Error = "Could not verify authentication"
		log.Println(err)
	}

	if ok {
		a.ServeInterface(w, r)
		return
	}

	// Validate authentication
	if r.Method == "POST" {
		err := ValidateRequestAuth(w, r, a.Config.AuthSecret, a.Config.AuthDigest)
		if err == nil {
			http.Redirect(w, r, a.Config.Base+"admin/", 302)
		} else {
			data.Error = err.Error()
			log.Println(err)
		}
	}

	a.ServeLogin(w, r, data)
}
