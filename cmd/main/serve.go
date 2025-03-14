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

// Configures guerilla daemon and starts it
func serve(cfile string) {
	// set logger
	logger := backends.Log()

	// create daemon and add custom processors
	d := guerrilla.Daemon{}
	d.AddProcessor("PSQL", backend.PSQLProcessor)
	d.AddProcessor("MimeParser", backend.MimeParserProcessor)

	// load configuration
	configFile := cfile
	_, err := d.LoadConfig(configFile)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %s\n", err)
	}

	// create a directory for logs if needed
	// the daemon will fail to start if the path to the log file as specified in `log_file` doesn't exist
	cfg := d.Config.LogFile
	if cfg != "stdout" && cfg != "stderr" && cfg != "off" {
		ensureLogDirectory(cfg)
	}

	// start daemon
	err = d.Start()
	if err != nil {
		logger.Fatalf("Failed to start daemon: %s\n", err)
	}

	// call signal handler
	sigHandler()
}

// Checks if the directory path to the file exists. If not, creates the directory path
func ensureLogDirectory(logfile string) {
	// extract the directory path from the logfile
	dir := filepath.Dir(logfile)

	// check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// create the directory with appropriate permissions
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			logger.Fatalf("failed to create directory: %s", err)
		}
	}
}
