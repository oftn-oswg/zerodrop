package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/jinzhu/configor"
)

// ZerodropConfig holds the configuration for an application instance.
type ZerodropConfig struct {
	Listen     string `default:"8080"`
	Base       string `default:"/"`
	AuthSecret string `default:"ggVUtPQdIL3kuMSeHQgn7PW9nv3XuJBp"`
	AuthDigest string `default:"11a55ac5de2beb9146e01386dd978a13bb9b99388f5eb52e37f69a32e3d5f11e"`
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

	notfound := NotFoundHandler{}

	if err := db.Connect(); err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/admin/", NewAdminHandler(&db, &config))
	http.Handle("/", &ShotHandler{DB: &db, Config: &config, NotFound: notfound})
	http.Serve(socket, nil)
}
