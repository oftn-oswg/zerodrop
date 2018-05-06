package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
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

// AdminPageData represents the data served to the admin templates.
type AdminPageData struct {
	Error    string
	Title    string
	LoggedIn bool
	Config   *ZerodropConfig
	Entries  []ZerodropEntry
}

// ServeLogin renders the login page.
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

// ServeNew renders the new entry page.
func (a *AdminHandler) ServeNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseMultipartForm(int64(a.Config.UploadMaxSize))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		entry := &ZerodropEntry{Creation: time.Now()}

		// Publish information
		entry.Name = r.FormValue("publish")
		if entry.Name == "" {
			id, err := uuid.NewV4()
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			entry.Name = id.String()
		}

		// Source information
		switch source := r.FormValue("source"); source {
		case "url":
			entry.URL = r.FormValue("url")
			entry.Redirect = r.FormValue("url_type") == "redirect"

		case "file":
			file, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer file.Close()

			filename := url.PathEscape(entry.Name)
			fullpath := filepath.Join(a.Config.UploadDirectory, filename)

			perms := os.FileMode(a.Config.UploadPermissions)
			out, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE, perms)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer out.Close()

			_, err = io.Copy(out, file)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			entry.Filename = filename
			entry.ContentType = r.FormValue("file_type")

		case "text":
			filename := url.PathEscape(entry.Name)
			fullpath := filepath.Join(a.Config.UploadDirectory, filename)

			perms := os.FileMode(a.Config.UploadPermissions)
			out, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE, perms)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer out.Close()

			_, err = io.WriteString(out, r.FormValue("text"))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			entry.Filename = filename
			entry.ContentType = r.FormValue("text_type")

		default:
			http.Error(w, "Source selection must be url, file, or text", 500)
			return
		}

		// Access information
		entry.AccessExpire = r.FormValue("access_expire") != ""
		entry.AccessExpireCount, _ = strconv.Atoi(r.FormValue("access_expire_count"))
		entry.AccessBlacklist = ParseBlacklist(r.FormValue("blacklist"), a.Config.Databases)

		if err := a.DB.Create(entry); err == nil {
			log.Printf("Created entry %s", entry)
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

// ServeMain serves the entry list.
func (a *AdminHandler) ServeMain(w http.ResponseWriter, r *http.Request) {
	data := &AdminPageData{Title: "Zerodrop Admin", LoggedIn: true, Config: a.Config}

	if r.Method == "POST" {
		r.ParseForm()

		switch r.FormValue("action") {

		case "train":
			name := r.FormValue("name")
			entry, ok := a.DB.Get(name)
			if ok {
				entry.SetTraining(!entry.AccessTrain)
			}

		case "delete":
			name := r.FormValue("name")
			if name != "" {
				a.DB.Remove(name)
				log.Printf("Removed entry: %s", name)
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
	a.Mux.ServeHTTP(w, r)
}
