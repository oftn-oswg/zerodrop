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
}

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

	if err := db.Connect(); err != nil {
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

	http.Handle("/admin/", http.StripPrefix("/admin", authHandler))
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))
	http.Handle("/", NewShotHandler(&db, &config, notfound))
	http.Serve(socket, nil)
}
