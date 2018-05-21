package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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
func NewAdminHandler(app *ZerodropApp) (*AdminHandler, error) {
	templateDirectory := "./templates"
	staticDirectory := "./static"

	handler := &AdminHandler{DB: app.DB, Config: app.Config}

	// Load templates
	templateFiles := []string{}
	files, err := ioutil.ReadDir(templateDirectory)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		filename := file.Name()
		if strings.HasSuffix(filename, ".tmpl") {
			templateFiles = append(templateFiles,
				path.Join(templateDirectory, filename))
		}
	}

	handler.Templates, err = template.ParseFiles(templateFiles...)
	if err != nil {
		return nil, err
	}

	// Create ServeMux
	mux := http.NewServeMux()
	mux.Handle("/admin/static/",
		http.StripPrefix("/admin/static", http.FileServer(http.Dir(staticDirectory))))
	mux.HandleFunc("/admin/new", handler.ServeNew)
	mux.HandleFunc("/admin/", handler.ServeMain)
	handler.Mux = mux

	return handler, nil
}

// AdminPageData represents the data served to the admin templates.
type AdminPageData struct {
	Error    string
	Title    string
	LoggedIn bool
	Config   *ZerodropConfig
	Entries  []*ZerodropEntry
}

// ServeLogin renders the login page.
func (a *AdminHandler) ServeLogin(w http.ResponseWriter, r *http.Request) {
	page := a.Config.Base + "admin/login"
	if r.URL.Path != "/login" {
		http.Redirect(w, r, page, 302)
		return
	}

	data := &AdminPageData{Title: "Zerodrop Login", Config: a.Config}
	loginTmpl := a.Templates.Lookup("login.tmpl")
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
		entry.AccessBlacklist = ParseBlacklist(r.FormValue("blacklist"), a.Config.IPCat)
		entry.AccessRedirectOnDeny = strings.TrimSpace(r.FormValue("access_redirect_on_deny"))

		if err := a.DB.Update(entry); err != nil {
			log.Printf("Error creating entry %s: %s", entry.Name, err)
		} else {
			log.Printf("Created entry %s", entry)
		}

		http.Redirect(w, r, a.Config.Base+"admin/", 302)
		return
	}

	data := &AdminPageData{Title: "Zerodrop New", LoggedIn: true, Config: a.Config}
	loginTmpl := a.Templates.Lookup("new.tmpl")
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
			entry, err := a.DB.Get(name)
			if err != nil {
				log.Println(err)
			} else {
				entry.SetTraining(!entry.AccessTrain)
				if err := a.DB.Update(entry); err != nil {
					log.Println(err)
				}
			}

		case "delete":
			name := r.FormValue("name")
			if name != "" {
				err := a.DB.Remove(name)
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("Removed entry: %s", name)
				}
			}

		case "clear":
			err := a.DB.Clear()
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Cleared all entries")
			}

		}

		http.Redirect(w, r, a.Config.Base+"admin/", 302)
		return
	}

	var err error
	data.Entries, err = a.DB.List()

	interfaceTmpl := a.Templates.Lookup("main.tmpl")
	if interfaceTmpl.ExecuteTemplate(w, "main", data) != nil {
		log.Println(err)
	}
}

func (a *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Mux.ServeHTTP(w, r)
}
