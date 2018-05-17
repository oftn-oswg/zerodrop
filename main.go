package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"

	"github.com/jinzhu/configor"
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

	RedirectLevels       int    `default:"128"`
	RedirectSelfDestruct string `default:"\U0001f4a3"` // Bomb emoji

	SelfDestruct []string

	DB struct {
		Driver string `default:"sqlite3"`
		Source string `default:"zerodrop.db"`
	}
}

type Signal int

const (
	GracefulShutdown Signal = iota
	SelfDestruct
)

func main() {
	var db ZerodropDB
	var config ZerodropConfig
	var configFile string

	// Parse configuration file from command line
	flag.StringVar(&configFile, "config", "/etc/zerodrop/config.yml",
		"Location of the configuration file")
	flag.Parse()

	// Population configuration struct
	configor.Load(&config, configFile)
	log.Printf("Loaded configuration: %#v", config)

	network, address := ParseSocketName(config.Listen)

	if network == "unix" {
		os.Remove(address)
	}

	socket, err := net.Listen(network, address)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()

	if network == "unix" {
		if config.Group != "" {
			uid := os.Geteuid()
			group, err := user.LookupGroup(config.Group)
			if err != nil {
				log.Fatal(err)
			}
			gid, err := strconv.Atoi(group.Gid)
			if err != nil {
				log.Fatal(err)
			}
			if err := os.Chown(address, uid, gid); err != nil {
				log.Fatal(err)
			}
		}
		if err := os.Chmod(address, 0660); err != nil {
			log.Fatal(err)
		}
	}

	notfound := NotFoundHandler{}

	if err := db.Connect(config.DB.Driver, config.DB.Source); err != nil {
		log.Fatal(err)
	}

	adminHandler := NewAdminHandler(&db, &config)

	authHandler := &AuthHandler{
		Credentials: AuthCredentials{
			Digest: config.AuthDigest,
			Secret: []byte(config.AuthSecret),
		},

		Success: adminHandler,
		Failure: http.HandlerFunc(adminHandler.ServeLogin),

		FailureRedirect: config.Base + "admin/login?err=1",
		SuccessRedirect: config.Base + "admin/",
	}

	signals := make(chan Signal)

	http.Handle("/admin/", http.StripPrefix("/admin", authHandler))
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
	http.Handle("/", NewShotHandler(&db, &config, notfound, signals))
	go http.Serve(socket, nil)

	signal := <-signals
	switch signal {
	case GracefulShutdown:
		return
	case SelfDestruct:
		selfdestroy(&config)
	}
}

func selfdestroy(config *ZerodropConfig) {
	errors := []string{}
	tag := "SELF-DESTRUCT"

	log.Printf("%s: initiating!", tag)

	// Copy removals list
	removals := make([]string, len(config.SelfDestruct))
	copy(removals, config.SelfDestruct)

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
