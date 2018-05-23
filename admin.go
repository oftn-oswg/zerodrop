package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"

	uuid "github.com/satori/go.uuid"

	"github.com/oftn-oswg/secureform"

	"gopkg.in/ezzarghili/recaptcha-go.v2"
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
	Admin bool   `json:"admin"`
	Token string `json:"token"`
	jwt.StandardClaims
}

// AdminPageData represents the data served to the admin templates.
type AdminPageData struct {
	Error  string
	Title  string
	Claims *AdminClaims
	Config *ZerodropConfig

	All     bool
	Entries []*ZerodropEntry
}

type AdminFormCredentials struct {
	Credentials string `form:"credentials"`
}

type EntrySource int

const (
	EntrySourceURL EntrySource = iota
	EntrySourceFile
	EntrySourceText
)

func (s *EntrySource) Set(value string) error {
	switch value {
	case "url":
		*s = EntrySourceURL
	case "file":
		*s = EntrySourceFile
	case "text":
		*s = EntrySourceText
	default:
		return errors.New("Source type can be only url, file, or text")
	}
	return nil
}

type RequestURI string

func (u *RequestURI) Set(value string) error {
	if value != "" {
		_, err := url.ParseRequestURI(value)
		if err != nil {
			return err
		}
	}

	*u = RequestURI(value)
	return nil
}

type EntryRedirect bool

func (f *EntryRedirect) Set(value string) error {
	switch value {
	case "redirect":
		*f = true
	case "proxy":
		*f = false
	default:
		return errors.New("Invalid url type")
	}
	return nil
}

type ContentType string

func (f *ContentType) Set(value string) error {
	if value != "" {
		_, _, err := mime.ParseMediaType(value)
		if err != nil {
			return err
		}
	} else {
		value = "text/plain"
	}
	*f = ContentType(value)
	return nil
}

type PageAction int

const (
	PageActionClear PageAction = iota
	PageActionDelete
	PageActionTrain
)

func (s *PageAction) Set(value string) error {
	switch value {
	case "clear":
		*s = PageActionClear
	case "delete":
		*s = PageActionDelete
	case "train":
		*s = PageActionTrain
	default:
		return errors.New("Page action must be clear, delete, or train")
	}
	return nil
}

type AdminFormNewEntry struct {
	// Publish information
	Name  string `form:"publish?max=512"`
	Token string `form:"token?max=64"`

	// Source information
	Source EntrySource `form:"source"`

	URL      RequestURI    `form:"url"`
	Redirect EntryRedirect `form:"url_type"`

	File     secureform.File `form:"file"`
	FileType ContentType     `form:"file_type"`

	Text     string      `form:"text"`
	TextType ContentType `form:"text_type"`

	// Access information
	AccessExpire         bool   `form:"access_expire"`
	AccessExpireCount    uint   `form:"access_expire_count"`
	AccessBlacklist      string `form:"blacklist"`
	AccessRedirectOnDeny string `form:"access_redirect_on_deny?max=512"`

	// ReCaptcha
	ReCaptchaResponse string `form:"g-recaptcha-response"`
}

type AdminFormPageAction struct {
	Action PageAction `form:"action"`
	Name   string     `form:"name?max=512"`
	Token  string     `form:"token?max=64"`
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
	handler.HandleFunc("/admin/my", handler.ServeList)
	handler.HandleFunc("/admin/", handler.ServeList)

	return handler, nil
}

// verify returns any claims present in the request
func (a *AdminHandler) verify(w http.ResponseWriter, r *http.Request) (*AdminClaims, error) {
	if cookie, err := r.Cookie("jwt"); err == nil {
		token, err := jwt.ParseWithClaims(cookie.Value, &AdminClaims{},
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(a.App.Config.AuthSecret), nil
			})
		if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
			return claims, nil
		}
		return nil, fmt.Errorf("Unknown error parsing validation cookie: %s", err.Error())
	}

	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return nil, err
	}

	claims := &AdminClaims{
		Admin: false,
		Token: hex.EncodeToString(token),
	}

	a.setClaims(w, claims)

	return claims, nil
}

// validate scans the request for credentials and generates a auth token
func (a *AdminHandler) validate(w http.ResponseWriter, r *http.Request) error {
	form := AdminFormCredentials{}
	memory := int64(1 << 10) // 1 kB
	p := secureform.NewParser(memory, memory, memory)
	err := p.Parse(w, r, &form)
	if err != nil {
		return err
	}

	validDigestBytes, err := hex.DecodeString(a.App.Config.AuthDigest)
	if err != nil {
		return err
	}

	digest := sha256.Sum256([]byte(form.Credentials))

	num, err := rand.Int(rand.Reader, big.NewInt(2000))
	if err != nil {
		return err
	}

	time.Sleep(2*time.Second + time.Duration(num.Int64()-1000)*time.Millisecond)
	if subtle.ConstantTimeCompare(validDigestBytes, digest[:]) == 1 {

		// Authentication successful; set cookie
		claims := &AdminClaims{Admin: true}
		err := a.setClaims(w, claims)
		if err != nil {
			return err
		}

		return nil
	}

	return errors.New("Invalid password")
}

func (a *AdminHandler) setClaims(w http.ResponseWriter, claims *AdminClaims) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.App.Config.AuthSecret))
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "jwt",
		Value:   tokenString,
		Expires: time.Now().Add(365 * 24 * time.Hour), // 1 year
	})

	return nil
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

	claims, _ := a.verify(w, r)
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
	claims, _ := a.verify(w, r)

	if r.Method == "POST" {
		form := AdminFormNewEntry{}

		p := secureform.NewParser(
			int64(a.App.Config.UploadMaxSize),
			int64(a.App.Config.UploadMaxSize),
			0)
		err := p.Parse(w, r, &form)

		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Test ReCaptcha
		if a.App.Config.Recaptcha.SiteKey != "" {
			captcha, err := recaptcha.NewReCAPTCHA(a.App.Config.Recaptcha.SecretKey)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), 500)
				return
			}
			ok, err := captcha.Verify(form.ReCaptchaResponse, RealRemoteIP(r).String())
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if !ok {
				http.Error(w, "You might be a robot. Rust in peace.", 500)
				return
			}
		}

		entry := &ZerodropEntry{Creation: time.Now()}

		// Publish information
		entry.Name = form.Name
		if entry.Name == "" {
			id, err := uuid.NewV4()
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			entry.Name = id.String()
		}

		// Source information
		switch form.Source {
		case EntrySourceURL:
			entry.URL = string(form.URL)
			entry.Redirect = bool(form.Redirect)

		case EntrySourceFile:
			file, err := form.File.Open()
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
			entry.ContentType = string(form.FileType)

		case EntrySourceText:
			filename := url.PathEscape(entry.Name)
			fullpath := filepath.Join(a.App.Config.UploadDirectory, filename)

			perms := os.FileMode(a.App.Config.UploadPermissions)
			out, err := os.OpenFile(fullpath, os.O_WRONLY|os.O_CREATE, perms)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer out.Close()

			_, err = io.WriteString(out, form.Text)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			entry.Filename = filename
			entry.ContentType = string(form.TextType)
		}

		// Access information
		entry.AccessExpire = form.AccessExpire
		entry.AccessExpireCount = int(form.AccessExpireCount)
		entry.AccessBlacklist = ParseBlacklist(form.AccessBlacklist, a.App.Config.IPCat)
		entry.AccessRedirectOnDeny = strings.TrimSpace(form.AccessRedirectOnDeny)

		if err := a.App.DB.Update(entry, claims); err != nil {
			log.Printf("Error creating entry %s: %s", entry.Name, err)
		} else {
			log.Printf("Created entry %s", entry)
		}

		redirectPage := a.App.Config.Base + "admin/my"
		if claims.Admin {
			redirectPage = a.App.Config.Base + "admin/"
		}
		http.Redirect(w, r, redirectPage, 302)
		return
	}

	data := &AdminPageData{Title: "Zerodrop Admin :: New", Claims: claims, Config: a.App.Config}
	loginTmpl := a.Templates.Lookup("new.tmpl")
	err := loginTmpl.ExecuteTemplate(w, "new", data)
	if err != nil {
		log.Println(err)
	}
}

// ServeList serves the entry list.
func (a *AdminHandler) ServeList(w http.ResponseWriter, r *http.Request) {
	claims, _ := a.verify(w, r)
	data := &AdminPageData{Title: "Zerodrop Admin", Claims: claims, Config: a.App.Config}

	all := true
	if strings.HasSuffix(r.RequestURI, "/my") {
		all = false
	}

	if r.Method == "POST" {
		form := AdminFormPageAction{}
		mem := int64(1 << 10) // 1 kB
		p := secureform.NewParser(mem, mem, mem)
		err := p.Parse(w, r, &form)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		switch form.Action {

		case PageActionTrain:
			entry, err := a.App.DB.Get(form.Name)
			if err != nil {
				log.Println(err)
			} else {
				entry.SetTraining(!entry.AccessTrain)
				if err := a.App.DB.Update(entry, claims); err != nil {
					log.Println(err)
				}
			}

		case PageActionDelete:
			if form.Name != "" {
				err := a.App.DB.Remove(form.Name, claims)
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("Removed entry: %s", form.Name)
				}
			}

		case PageActionClear:
			err := a.App.DB.Clear(claims)
			if err != nil {
				log.Println(err)
			} else {
				log.Printf("Cleared all entries with token %q", form.Token)
			}

		}

		http.Redirect(w, r, r.RequestURI, 302)
		return
	}

	token := ""
	if !all {
		token = claims.Token
	}
	entries, err := a.App.DB.List(token)
	if err != nil {
		log.Println(err)
	}

	data.All = all
	data.Entries = entries

	interfaceTmpl := a.Templates.Lookup("entries.tmpl")
	err = interfaceTmpl.ExecuteTemplate(w, "entries", data)
	if err != nil {
		log.Println(err)
	}
}
