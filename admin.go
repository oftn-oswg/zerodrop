package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"

	uuid "github.com/satori/go.uuid"
)

// AdminHandler serves the administration page, or asks for credentials if not
// already authenticated.
type AdminHandler struct {
	http.ServeMux

	App       *ZerodropApp
	Templates *template.Template
}

// AdminClaims represents the claims of the JWT (JSON Web Token)
type AdminClaims struct {
	Admin bool `json:"admin"`
	jwt.StandardClaims
}

// AdminPageData represents the data served to the admin templates.
type AdminPageData struct {
	Error   string
	Title   string
	Claims  *AdminClaims
	Config  *ZerodropConfig
	Entries []*ZerodropEntry
}

// NewAdminHandler creates a new admin handler with the specified configuration
// and loads the template files into cache.
func NewAdminHandler(app *ZerodropApp) (*AdminHandler, error) {
	templateDirectory := "./templates"
	staticDirectory := "./static"

	handler := &AdminHandler{App: app}

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

	handler.Handle("/admin/static/",
		http.StripPrefix("/admin/static", http.FileServer(http.Dir(staticDirectory))))
	handler.HandleFunc("/admin/login", handler.ServeLogin)
	handler.HandleFunc("/admin/logout", handler.ServeLogout)
	handler.HandleFunc("/admin/new", handler.ServeNew)
	handler.HandleFunc("/admin/my", handler.ServeMy)
	handler.HandleFunc("/admin/", handler.ServeMain)

	return handler, nil
}

// verify returns any claims present in the request
func (a *AdminHandler) verify(r *http.Request) (*AdminClaims, error) {
	if cookie, err := r.Cookie("jwt"); err == nil {
		token, err := jwt.ParseWithClaims(cookie.Value, &AdminClaims{},
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(a.App.Config.AuthSecret), nil
			})
		if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
			if claims.Admin {
				return claims, nil
			}
		} else {
			return nil, fmt.Errorf("Unknown error parsing validation cookie: %s", err.Error())
		}
	}

	return nil, nil
}

// validate scans the request for credentials and generates a auth token
func (a *AdminHandler) validate(w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

	creds, ok := r.Form["credentials"]
	if !ok || len(creds) < 1 {
		return errors.New("No credentials provided")
	}

	validDigestBytes, err := hex.DecodeString(a.App.Config.AuthDigest)
	if err != nil {
		return err
	}

	digest := sha256.Sum256([]byte(creds[0]))

	time.Sleep(2*time.Second + time.Duration(rand.Intn(2000)-1000)*time.Millisecond)
	if subtle.ConstantTimeCompare(validDigestBytes, digest[:]) == 1 {

		// Authentication successful; set cookie
		exp := time.Now().Add(time.Hour * time.Duration(24)).Unix()
		claims := AdminClaims{Admin: true, StandardClaims: jwt.StandardClaims{ExpiresAt: exp}}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(a.App.Config.AuthSecret))
		if err != nil {
			return err
		}

		http.SetCookie(w, &http.Cookie{
			Name:   "jwt",
			Value:  tokenString,
			MaxAge: 24 * 60 * 60, // 1 day
			// Secure: true,
		})

		return nil
	}

	return errors.New("Invalid password")
}

// ServeLogin renders the login page.
func (a *AdminHandler) ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		ip := RealRemoteIP(r)
		err := a.validate(w, r)
		if err != nil {
			// Failed authentication
			log.Printf("Failed authentication by %s: %s", ip, err)
			http.Redirect(w, r, "/admin/login?err=1", 302)
			return
		}

		// Successful authentication
		log.Printf("Successful authentication by %s", ip)
		http.Redirect(w, r, "/admin/", 302)
		return
	}

	claims, _ := a.verify(r)
	data := &AdminPageData{Title: "Zerodrop Login", Claims: claims, Config: a.App.Config}
	loginTmpl := a.Templates.Lookup("login.tmpl")
	err := loginTmpl.ExecuteTemplate(w, "login", data)
	if err != nil {
		log.Println(err)
	}
}

// ServeLogout renders the logout page.
func (a *AdminHandler) ServeLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "jwt",
		Value:   "",
		Expires: time.Unix(0, 0),
	})
	http.Redirect(w, r, "/admin/", 302)
}

// ServeNew renders the new entry page.
func (a *AdminHandler) ServeNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseMultipartForm(int64(a.App.Config.UploadMaxSize))
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
			fullpath := filepath.Join(a.App.Config.UploadDirectory, filename)

			perms := os.FileMode(a.App.Config.UploadPermissions)
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
			fullpath := filepath.Join(a.App.Config.UploadDirectory, filename)

			perms := os.FileMode(a.App.Config.UploadPermissions)
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
		entry.AccessBlacklist = ParseBlacklist(r.FormValue("blacklist"), a.App.Config.IPCat)
		entry.AccessRedirectOnDeny = strings.TrimSpace(r.FormValue("access_redirect_on_deny"))

		if err := a.App.DB.Update(entry); err != nil {
			log.Printf("Error creating entry %s: %s", entry.Name, err)
		} else {
			log.Printf("Created entry %s", entry)
		}

		http.Redirect(w, r, a.App.Config.Base+"admin/", 302)
		return
	}

	claims, _ := a.verify(r)
	data := &AdminPageData{Title: "Zerodrop Admin :: New", Claims: claims, Config: a.App.Config}
	loginTmpl := a.Templates.Lookup("new.tmpl")
	err := loginTmpl.ExecuteTemplate(w, "new", data)
	if err != nil {
		log.Println(err)
	}
}

// ServeMain serves the entry list.
func (a *AdminHandler) ServeMy(w http.ResponseWriter, r *http.Request) {
	claims, _ := a.verify(r)
	data := &AdminPageData{Title: "Zerodrop Admin", Claims: claims, Config: a.App.Config}

	if r.Method == "POST" {
		r.ParseForm()

		switch r.FormValue("action") {

		case "train":
			name := r.FormValue("name")
			entry, err := a.App.DB.Get(name)
			if err != nil {
				log.Println(err)
			} else {
				entry.SetTraining(!entry.AccessTrain)
				if err := a.App.DB.Update(entry); err != nil {
					log.Println(err)
				}
			}

		case "delete":
			name := r.FormValue("name")
			if name != "" {
				err := a.App.DB.Remove(name)
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("Removed entry: %s", name)
				}
			}

		case "clear":
			err := a.App.DB.Clear()
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Cleared all entries")
			}

		}

		http.Redirect(w, r, a.App.Config.Base+"admin/", 302)
		return
	}

	var err error
	data.Entries, err = a.App.DB.List()

	interfaceTmpl := a.Templates.Lookup("my.tmpl")
	if interfaceTmpl.ExecuteTemplate(w, "my", data) != nil {
		log.Println(err)
	}
}

// ServeMain serves the entry list.
func (a *AdminHandler) ServeMain(w http.ResponseWriter, r *http.Request) {
	claims, _ := a.verify(r)
	data := &AdminPageData{Title: "Zerodrop Admin", Claims: claims, Config: a.App.Config}

	if r.Method == "POST" {
		r.ParseForm()

		switch r.FormValue("action") {

		case "train":
			name := r.FormValue("name")
			entry, err := a.App.DB.Get(name)
			if err != nil {
				log.Println(err)
			} else {
				entry.SetTraining(!entry.AccessTrain)
				if err := a.App.DB.Update(entry); err != nil {
					log.Println(err)
				}
			}

		case "delete":
			name := r.FormValue("name")
			if name != "" {
				err := a.App.DB.Remove(name)
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("Removed entry: %s", name)
				}
			}

		case "clear":
			err := a.App.DB.Clear()
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Cleared all entries")
			}

		}

		http.Redirect(w, r, a.App.Config.Base+"admin/", 302)
		return
	}

	var err error
	data.Entries, err = a.App.DB.List()

	interfaceTmpl := a.Templates.Lookup("all.tmpl")
	if interfaceTmpl.ExecuteTemplate(w, "all", data) != nil {
		log.Println(err)
	}
}
