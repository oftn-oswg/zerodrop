package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jinzhu/configor"
)

func main() {
	var config ZerodropConfig
	var configFile string

	// Parse configuration file from command line
	flag.StringVar(&configFile, "config", "/etc/zerodrop/config.yml",
		"Location of the configuration file")
	flag.Parse()

	// Population configuration struct
	configor.Load(&config, configFile)
	log.Printf("Loaded configuration: %#v", config)

	app := NewZerodropApp(&config)
	err := app.Start()
	if err != nil {
		log.Fatal(err)
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	fmt.Printf("Received signal %s\n", sig)
	app.Stop()
}
