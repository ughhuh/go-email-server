// meat and potatoes

package main

import (
	"log"
	"os"

	"github.com/phires/go-guerrilla"
	"github.com/phires/go-guerrilla/backends"
	"github.com/ughhuh/go-email-server/backend"
)

var (
	signalChannel = make(chan os.Signal, 1) // a channel for storing signals
	d             guerrilla.Daemon
	logger        log.Logger
)

func init() {
	// confirm config file exists
}

func serve() {
	// start server
	// cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String()}

	d := guerrilla.Daemon{}
	d.AddProcessor("PSQL", backend.PSQL)

	_, err := d.LoadConfig("../../config.json")
	if err != nil {
		logger.Fatalf("Failed to load configuration: %s\n", err)
	}

	ensureLogDirectory()
	logger := backends.Log()

	err = d.Start()

	if err != nil {
		logger.Fatalf("Failed to start daemon: %s\n", err)
	}
	// check max clients is OK

	// call signal handler
	sigHandler()
}

func ensureLogDirectory() {
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		err := os.Mkdir("./logs", os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
	}
}
