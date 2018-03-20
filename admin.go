package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
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
	DB        *OneshotDB
	Config    *OneshotConfig
	Templates *template.Template
}

// NewAdminHandler creates a new admin handler with the specified configuration
// and loads the template files into cache.
func NewAdminHandler(db *OneshotDB, config *OneshotConfig) *AdminHandler {
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
	Config  *OneshotConfig
	Entries []OneshotEntry
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

		if r.FormValue("action") == "add" {
			accessExpiry, err := strconv.Atoi(r.FormValue("access_expiry"))
			if err != nil {
				accessExpiry = 0
			}

			entry := &OneshotEntry{
				URL:          r.FormValue("url"),
				Redirect:     r.FormValue("redirect") != "",
				Creation:     time.Now(),
				AccessExpiry: accessExpiry,
				AccessCount:  0,
			}

			if err := a.DB.Create(entry); err == nil {
				log.Printf("Added entry: %#v", entry)
			}
		}

		http.Redirect(w, r, a.Config.Base+"admin/", 302)
		return
	}

	data := &AdminPageData{}
	data.Title = "Oneshot Admin"
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
	data.Title = "Oneshot Login"
	data.Config = a.Config

	// Verify authentication
	if cookie, err := r.Cookie("jwt"); err == nil {
		token, err := jwt.ParseWithClaims(cookie.Value, &AdminClaims{},
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(a.Config.AuthSecret), nil
			})
		if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
			if claims.Admin {
				a.ServeInterface(w, r)
				return
			}
		} else {
			data.Error = "Unknown error parsing validation cookie"
			log.Println(err)
		}
	}

	// Validate authentication
	if r.Method == "POST" {
		r.ParseForm()

		creds, ok := r.Form["credentials"]
		if !ok || len(creds) < 1 {
			http.Redirect(w, r, a.Config.Base+"admin/", 302)
			return
		}

		validDigest, err := hex.DecodeString(a.Config.AuthDigest)
		if err != nil {
			data.Error = "Could not authenticate"
			log.Println(data.Error + ": " + err.Error())
			a.ServeLogin(w, r, data)
			return
		}

		digest := sha256.Sum256([]byte(creds[0]))

		time.Sleep(2*time.Second + time.Duration(rand.Intn(2000)-1000)*time.Millisecond)
		if subtle.ConstantTimeCompare(validDigest, digest[:]) == 1 {

			// Authentication successful; set cookie
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, AdminClaims{Admin: true})
			tokenString, err := token.SignedString([]byte(a.Config.AuthSecret))
			if err != nil {
				data.Error = "Could not validate authentication"
				log.Println(data.Error + ": " + err.Error())
				a.ServeLogin(w, r, data)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:   "jwt",
				Value:  tokenString,
				MaxAge: 24 * 60 * 60, // 1 day
				// Secure: true,
			})

			http.Redirect(w, r, a.Config.Base+"admin/", 302)
			return
		}

		data.Error = "Invalid password"
		log.Println("Invalid password '" + creds[0] + "' from IP " + RealRemoteAddr(r))
		a.ServeLogin(w, r, data)
		return
	}

	a.ServeLogin(w, r, data)
}
