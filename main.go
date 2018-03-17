package main

import (
	"flag"
	"log"
	"net"
	"net/http"

	"github.com/jinzhu/configor"
)

// OneshotConfig holds the configuration for an application instance.
type OneshotConfig struct {
	Listen     string `default:"8080"`
	AuthSecret string `default:"ggVUtPQdIL3kuMSeHQgn7PW9nv3XuJBp"`
	AuthDigest string `default:"11a55ac5de2beb9146e01386dd978a13bb9b99388f5eb52e37f69a32e3d5f11e"`
}

func main() {
	var db OneshotDB
	var config OneshotConfig
	var configFile string

	// Parse configuration file from command line
	flag.StringVar(&configFile, "config", "/etc/oneshot/config.yml",
		"Location of the configuration file")
	flag.Parse()

	// Population configuration struct
	configor.Load(&config, configFile)

	network, address := ParseSocketName(config.Listen)
	socket, err := net.Listen(network, address)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()

	notfound := NotFoundHandler{}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/admin/", NewAdminHandler(&config))
	http.Handle("/", &ShotHandler{DB: &db, Config: &config, NotFound: notfound})
	http.Serve(socket, nil)
}
