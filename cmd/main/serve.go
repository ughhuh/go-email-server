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
	configFile    string
)

func serve(cfile string, ldir string) {
	// start server
	// cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String()}
	ensureLogDirectory(ldir)
	logger := backends.Log()

	configFile := cfile
	d := guerrilla.Daemon{}
	d.AddProcessor("PSQL", backend.PSQL)

	_, err := d.LoadConfig(configFile)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %s\n", err)
	}
	err = d.Start()

	if err != nil {
		logger.Fatalf("Failed to start daemon: %s\n", err)
	}
	// check max clients is OK

	// call signal handler
	sigHandler()
}

func ensureLogDirectory(logdir string) {
	if _, err := os.Stat(logdir); os.IsNotExist(err) {
		err := os.Mkdir(logdir, os.ModePerm)
		if err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
	}
}
