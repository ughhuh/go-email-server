// meat and potatoes

package main

import (
	"os"
)

var (
	signalChannel = make(chan os.Signal, 1) // a channel for storing signals
)

func init() {
	// confirm config file exists
}

func serve() {
	// start server

	// check max clients is OK

	// call signal handler
	sigHandler()
}
