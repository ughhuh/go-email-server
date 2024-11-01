// meat and potatoes

package main

import (
	"log"
	"os"
	"path/filepath"

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

func serve(cfile string) {
	// start server
	// cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String()}
	logger := backends.Log()

	configFile := cfile
	d := guerrilla.Daemon{}
	d.AddProcessor("PSQL", backend.PSQL)
	d.AddProcessor("MimeParser", backend.MimeParser)

	_, err := d.LoadConfig(configFile)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %s\n", err)
	}

	ensureLogDirectory(d.Config.LogFile)

	err = d.Start()

	if err != nil {
		logger.Fatalf("Failed to start daemon: %s\n", err)
	}
	// check max clients is OK

	// call signal handler
	sigHandler()
}

func ensureLogDirectory(logfile string) {
	// Extract the directory path from the logfile
	dir := filepath.Dir(logfile)

	// Check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Create the directory with appropriate permissions
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			logger.Fatalf("failed to create directory: %s", err)
		}
	}
}
