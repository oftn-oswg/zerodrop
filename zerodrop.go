package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/oftn-oswg/socket"
)

// ZerodropConfig holds the configuration for an application instance.
type ZerodropConfig struct {
	Listen string `default:"8080"`
	Group  string

	Base       string `default:"/"`
	AuthSecret string `default:"ggVUtPQdIL3kuMSeHQgn7PW9nv3XuJBp"`
	AuthDigest string `default:"11a55ac5de2beb9146e01386dd978a13bb9b99388f5eb52e37f69a32e3d5f11e"`

	GeoDB string
	IPCat map[string]string

	UploadDirectory   string `default:"."`
	UploadPermissions uint32 `default:"0600"`
	UploadMaxSize     uint64 `default:"1000000"`

	SelfDestruct struct {
		Enable  bool   `default:"false"`
		Keyword string `default:"\U0001f4a3"` // Bomb emoji
		Files   []string
	}

	RedirectLevels int `default:"128"`

	Recaptcha struct {
		SiteKey   string
		SecretKey string
	}

	DB struct {
		Driver string `default:"sqlite3"`
		Source string `default:"zerodrop.db"`
	}
}

type ZerodropApp struct {
	Config *ZerodropConfig
	Server *http.Server
	DB     *ZerodropDB

	AdminHandler *AdminHandler
	ShotHandler  *ShotHandler
	NotFound     *NotFoundHandler
}

func NewZerodropApp(config *ZerodropConfig) (app *ZerodropApp, err error) {
	app = &ZerodropApp{
		Config: config,
		Server: &http.Server{},
		DB:     &ZerodropDB{},
	}

	app.AdminHandler, err = NewAdminHandler(app)
	if err != nil {
		return nil, err
	}
	app.ShotHandler = NewShotHandler(app)
	app.NotFound = &NotFoundHandler{}

	return app, nil
}

func (z *ZerodropApp) Start() error {
	config := z.Config
	db := z.DB

	network, address := socket.Parse(config.Listen)
	socket, err := socket.Listen(network, address, 0660)
	if err != nil {
		return err
	}

	if err := db.Connect(config.DB.Driver, config.DB.Source); err != nil {
		return err
	}

	rootserver := http.FileServer(http.Dir("./static/root/"))

	mux := http.NewServeMux()
	mux.Handle("/", z.ShotHandler)
	mux.Handle("/admin/", z.AdminHandler)
	mux.Handle("/robots.txt", rootserver)
	mux.Handle("/favicon.ico", rootserver)

	z.Server.Handler = mux

	go z.Server.Serve(socket)

	return nil
}

func (z *ZerodropApp) Stop() {
	z.Server.Shutdown(context.Background())
}

func (z *ZerodropApp) SelfDestruct() {
	if !z.Config.SelfDestruct.Enable {
		return
	}

	config := z.Config
	errors := []string{}
	tag := "SELF-DESTRUCT"

	log.Printf("%s: initiating!", tag)

	// Copy removals list
	removals := make([]string, len(config.SelfDestruct.Files))
	copy(removals, config.SelfDestruct.Files)

	// Prepend uploads directory
	removals = append([]string{config.UploadDirectory}, removals...)

	// Prepend this binary
	exec, err := os.Executable()
	if err != nil {
		errors = append(errors, "zerodrop binary: "+os.Args[0])
		log.Printf("%s: could not locate binary! %s", tag, err)
	} else {
		removals = append([]string{exec}, removals...)
	}

	for _, removal := range removals {
		err := os.RemoveAll(removal)
		if err != nil {
			errors = append(errors, removal)
			log.Printf("%s: %s", tag, err)
		}
	}

	if len(errors) > 0 {
		log.Printf("%s: Encountered errors with the following files; please remove manually.", tag)
		for _, err := range errors {
			log.Printf("%s: - %s", tag, err)
		}
	}

	log.Printf("%s: shutting down!", tag)
	os.Exit(3)
}
